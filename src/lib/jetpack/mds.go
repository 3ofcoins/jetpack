package jetpack

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"code.google.com/p/go-uuid/uuid"

	"github.com/juju/errors"

	"lib/run"
	"lib/ui"
)

type MDSInfo struct {
	Pid, Uid, Gid, Port int
	Version, IP         string
}

func (mdsi *MDSInfo) String() string {
	return fmt.Sprintf("MDS[%d] (u%d g%d %v:%d %v)",
		mdsi.Pid, mdsi.Uid, mdsi.Gid, mdsi.IP, mdsi.Port, mdsi.Version)
}

func (h *Host) GetMDSUGID() (int, int) {
	if h.mdsUid < 0 {
		u, err := user.Lookup(Config().MustGetString("mds.user"))
		if err != nil {
			panic(err)
		}
		h.mdsUid, err = strconv.Atoi(u.Uid)
		if err != nil {
			panic(err)
		}
		h.mdsGid, err = strconv.Atoi(u.Gid)
		if err != nil {
			panic(err)
		}
	}
	return h.mdsUid, h.mdsGid
}

var metadataTokenSecret []byte

func metadataToken(id uuid.UUID) []byte {
	if id == nil {
		return nil
	}

	if metadataTokenSecret == nil {
		if hexSecret, ok := Config().Get("mds.token-key"); !ok {
			metadataTokenSecret = []byte{}
		} else if secret, err := hex.DecodeString(hexSecret); err != nil {
			panic(err)
		} else {
			metadataTokenSecret = secret
		}
	}

	if len(metadataTokenSecret) == 0 {
		return nil
	}

	h := hmac.New(sha512.New, metadataTokenSecret)
	h.Write(id)
	return h.Sum(nil)
}

// MetadataToken returns a hex-encoded metadata token for a UUID. If
// config property `mds.token-secret` is unset or id is nil, returns
// nil.
func MetadataToken(id uuid.UUID) string {
	if tk := metadataToken(id); tk == nil {
		return ""
	} else {
		return hex.EncodeToString(tk)
	}
}

// VerifyMetadataToken verifies a received token for UUID. If id is
// empty or config property `mds.token-secret` is unset, `received`
// should be an empty string.
func VerifyMetadataToken(id uuid.UUID, received string) bool {
	expected := metadataToken(id)
	if expected == nil {
		return received == ""
	}

	if len(received) != sha512.Size*2 {
		// Not a proper SHA-512 checksum; sanity check before decoding hex
		// to avoid trying to parse obviously wrong (and possibly
		// malicious) tokens
		return false
	}
	if receivedBytes, err := hex.DecodeString(received); err != nil {
		return false
	} else {
		return hmac.Equal(receivedBytes, expected)
	}
}

func (h *Host) metadataURLBase() (string, error) {
	if hostip, _, err := h.HostIP(); err != nil {
		return "", errors.Trace(err)
	} else {
		url := fmt.Sprintf("http://%v", hostip)
		if mdport := Config().MustGetInt("mds.port"); mdport != 80 {
			url = fmt.Sprintf("%v:%v", url, mdport)
		}
		return url, nil
	}
}

// MetadataURL returns URL of the metadata service for pod with
// provided UUID.
func (h *Host) MetadataURL(id uuid.UUID) (string, error) {
	if url, err := h.metadataURLBase(); err != nil {
		return "", errors.Trace(err)
	} else {
		if token := MetadataToken(id); token != "" {
			url = fmt.Sprintf("%v/~%v", url, token)
		}
		return url, nil
	}
}

func (h *Host) GetMDSInfo() (*MDSInfo, error) {
	var mdsi MDSInfo

	url, err := h.MetadataURL(uuid.NIL)
	if err != nil {
		return nil, errors.Trace(err)
	}

	resp, err := http.Get(url + "/_info")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("%v %v", resp.Proto, resp.Status)
	}

	mdsiJSON, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = json.Unmarshal(mdsiJSON, &mdsi)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &mdsi, nil
}

func (h *Host) validateMDSInfo(mdsi *MDSInfo) error {
	if mdsi.Version != Version() {
		return errors.Errorf("Version mismatch: ours %v, mds %v", Version(), mdsi.Version)
	}

	uid, gid := h.GetMDSUGID()

	if mdsi.Uid != uid {
		return errors.Errorf("UID mismatch: should be %d, is %d", uid, mdsi.Uid)
	}

	if mdsi.Gid != gid {
		return errors.Errorf("GID mismatch: should be %d, is %d", gid, mdsi.Gid)
	}

	if port := Config().MustGetInt("mds.port"); mdsi.Port != port {
		return errors.Errorf("Port mismatch: expected %d, got %d", port, mdsi.Port)
	}

	if hostip, _, err := h.HostIP(); err != nil {
		return errors.Trace(err)
	} else if ip := hostip.String(); mdsi.IP != ip {
		return errors.Errorf("IP mismatch: expected %v, got %v", ip, mdsi.IP)
	}

	return nil
}

// Returns: (nil, err) if MDS can't be contacted for info; (info, err)
// if info was wrong; (info, nil) if everything's fine.
func (h *Host) checkMDS() (*MDSInfo, error) {
	mdsi, err := h.GetMDSInfo()
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = h.validateMDSInfo(mdsi)
	if err != nil {
		return mdsi, err
	}

	return mdsi, nil
}

func (h *Host) startMDS() (*MDSInfo, error) {
	var log *os.File
	if logPath := Config().GetString("mds.logfile", ""); logPath != "" && logPath != "/dev/null" {
		if lf, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600); err != nil {
			return nil, errors.Trace(err)
		} else {
			log = lf
		}
	}

	cmd := run.Command("/usr/sbin/daemon", append([]string{
		"-u", Config().MustGetString("mds.user"),
		filepath.Join(Config().MustGetString("path.libexec"), "mds")},
		ConfigFlags()...)...)
	cmd.Cmd.Stdin = nil
	cmd.Cmd.Stdout = log
	cmd.Cmd.Stderr = log
	cmd.Cmd.Dir = "/"

	if err := cmd.Run(); err != nil {
		return nil, errors.Trace(err)
	}

	// Wait for port...
	addr, err := h.MetadataURL(uuid.NIL)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Hack hack hack: we just strip the "http://" prefix
	addr = addr[7:]

	spin := ui.NewSpinner("Waiting for MDS", ui.SuffixElapsed(), nil)
	defer spin.Finish()
	haveConnection := false
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if conn, err := net.Dial("tcp", addr); err != nil {
			spin.Step()
		} else {
			spin.Finish()
			conn.Close()
			haveConnection = true
			break
		}
	}

	if haveConnection {
		return h.checkMDS()
	} else {
		return nil, errors.New("Timeout waiting for metadata service")
	}
}

func (h *Host) NeedMDS() (*MDSInfo, error) {
	if mdsi, err := h.checkMDS(); err != nil && mdsi == nil {
		h.ui.Println("Metadata service down:", err)
		if Config().MustGetBool("mds.autostart") {
			mdsi, err = h.startMDS()
		}
		return mdsi, errors.Trace(err)
	} else if err != nil {
		h.ui.Printf("ERROR: %v: %v", mdsi, err)
		return mdsi, errors.Trace(err)
	} else {
		h.ui.Debug(mdsi)
		return mdsi, nil
	}
}
