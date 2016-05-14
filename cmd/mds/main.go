package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/pborman/uuid"

	"github.com/3ofcoins/jetpack/lib/jetpack"
	"github.com/appc/spec/schema/types"
)

var Host *jetpack.Host

func getPod(ip string) *jetpack.Pod {
	for _, pod := range Host.Pods() {
		if podIp, _ := pod.Manifest.Annotations.Get("ip-address"); podIp == ip {
			return pod
		}
	}
	return nil
}

func clientIP(r *http.Request) string {
	return strings.SplitN(r.RemoteAddr, ":", 2)[0]
}

func resp200(v interface{}, ct string) (int, []byte, string) {
	return http.StatusOK, []byte(fmt.Sprint(v)), ct
}

func resp404() (int, []byte, string) {
	return http.StatusNotFound, nil, "text/plain"
}

func resp403() (int, []byte, string) {
	return http.StatusForbidden, nil, "text/plain"
}

func resp500(err error) (int, []byte, string) {
	return http.StatusInternalServerError, []byte(err.Error()), "text/plain"
}

// Returns path, token. If token is not provided, empty string is
// returned.
func extractToken(url string) (string, string) {
	if strings.HasPrefix(url, "/~") {
		pieces := strings.SplitN(url[2:], "/", 2)
		return "/" + pieces[1], pieces[0]
	} else {
		return url, ""
	}
}

func doServeMetadata(r *http.Request) (int, []byte, string) {
	if r.URL.Path == "/" {
		// Root URL. We introduce ourselves, no questions asked.
		return http.StatusOK, []byte(fmt.Sprintf("Jetpack metadata service version %v\n", jetpack.Version())), "text/plain; charset=us-ascii"
	}

	path, token := extractToken(r.URL.Path)
	r.RequestURI = path // Strip token from future logs

	if path == "/_info" {
		if !jetpack.VerifyMetadataToken(uuid.NIL, token) {
			return resp403()
		}
		r.URL.User = url.User("host")
		if body, err := json.Marshal(&Info); err != nil {
			return resp500(err)
		} else {
			return http.StatusOK, body, "application/json"
		}
	}

	// All other requests should be coming from a pod.
	pod := getPod(clientIP(r))
	if pod == nil {
		return http.StatusTeapot, []byte("You are not a real pod. For you, I am a teapot."), "text/plain; charset=us-ascii"
	}

	// hack hack hack
	r.URL.User = url.User(pod.UUID.String())

	if !jetpack.VerifyMetadataToken(pod.UUID, token) {
		return http.StatusTeapot, []byte("You are not a real pod. For you, I am a teapot."), "text/plain; charset=us-ascii"
	}

	if !strings.HasPrefix(path, "/acMetadata/v1/") {
		// Not a metadata service request.
		return resp404()
	}

	path = path[len("/acMetadata/v1/"):]
	switch {

	case path == "pod/uuid":
		return resp200(pod.UUID, "text/plain; charset=us-ascii")

	case path == "pod/manifest":
		// Pod manifest
		if manifestJSON, err := json.Marshal(pod.Manifest); err != nil {
			panic(err)
		} else {
			return http.StatusOK, manifestJSON, "application/json"
		}

	case path == "pod/hmac/sign":
		content := r.FormValue("content")
		if content == "text/plain" {
			return http.StatusBadRequest, []byte("content form value not found\n"), "text/plain"
		}
		h := hmac.New(sha512.New, SigningKey)
		h.Write(pod.UUID)
		h.Write([]byte(content))
		return resp200(hex.EncodeToString(h.Sum(nil)), "text/plain; charset=us-ascii")

	case path == "pod/hmac/verify":
		uuid := uuid.Parse(r.FormValue("uuid"))
		if uuid == nil {
			return http.StatusBadRequest, []byte(fmt.Sprintf("Invalid UUID: %#v\n", r.FormValue("uuid"))), "text/plain"
		}

		sig, err := hex.DecodeString(r.FormValue("signature"))
		if err != nil {
			return http.StatusBadRequest, []byte(fmt.Sprintf("Invalid signature: %#v\n", r.FormValue("signature"))), "text/plain"
		}

		content := r.FormValue("content")
		if content == "text/plain" {
			return http.StatusBadRequest, []byte("content form value not found\n"), "text/plain"
		}

		h := hmac.New(sha512.New, SigningKey)
		h.Write(uuid)
		h.Write([]byte(content))

		if hmac.Equal(sig, h.Sum(nil)) {
			return http.StatusOK, nil, "text/plain; charset=us-ascii"
		} else {
			return http.StatusForbidden, nil, "text/plain; charset=us-ascii"
		}

	case path == "pod/annotations":
		if annJSON, err := json.Marshal(pod.Manifest.Annotations); err != nil {
			panic(err)
		} else {
			return http.StatusOK, annJSON, "application/json"
		}
	case strings.HasPrefix(path, "apps/"):
		// App metadata.
		subpath := path[len("apps/"):]

		for _, app := range pod.Manifest.Apps {
			appPrefix := string(app.Name) + "/"
			if strings.HasPrefix(subpath, appPrefix) {
				switch appPath := subpath[len(appPrefix):]; appPath {

				case "image/id":
					return resp200(app.Image.ID, "text/plain; charset=us-ascii")

				case "image/manifest":
					if img, err := Host.GetImage(app.Image.ID, "", nil); err != nil {
						panic(err)
					} else if manifestJSON, err := json.Marshal(img.Manifest); err != nil {
						panic(err)
					} else {
						return http.StatusOK, manifestJSON, "application/json"
					}

				case "annotations":
					img, err := Host.GetImage(app.Image.ID, "", nil)
					if err != nil {
						panic(err)
					}

					anns := make(types.Annotations, len(img.Manifest.Annotations))
					copy(anns, img.Manifest.Annotations)
					for _, ann := range app.Annotations {
						anns.Set(ann.Name, ann.Value)
					}

					annsJSON, err := json.Marshal(anns)
					if err != nil {
						panic(err)
					}

					return http.StatusOK, annsJSON, "application/json"
				}
			}
		}
		return resp404()

	default:
		return resp404()
	}
}

func ServeMetadata(w http.ResponseWriter, r *http.Request) {
	status, body, content_type := doServeMetadata(r)

	if body == nil {
		body = []byte(http.StatusText(status) + "\n")
	}

	// log_format combined '$remote_addr - $remote_user [$time_local] ' '"$request" $status $body_bytes_sent ' '"$http_referer" "$http_user_agent"';
	remote_user := "-"
	if r.URL.User != nil {
		remote_user = r.URL.User.Username()
	}

	fmt.Printf("%v - %v [%v] \"%v %v\" %d %d \"-\" \"-\"\n",
		clientIP(r),
		remote_user,
		time.Now(),
		r.Method,
		r.RequestURI,
		status,
		len(body))

	if content_type == "" {
		content_type = "text/plain"
	}
	w.Header().Set("Content-Type", content_type)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		panic(err)
	}
}

var Info jetpack.MDSInfo
var SigningKey []byte

func main() {
	flag.Parse()

	Info.Pid = os.Getpid()
	Info.Uid = -1
	Info.Gid = -1
	Info.Version = jetpack.Version()

	if host, err := jetpack.NewHost(); err != nil {
		log.Fatalln("Error initializing host:", err)
	} else {
		Host = host
	}

	if hostip, _, err := Host.HostIP(); err != nil {
		panic(err)
	} else {
		Info.IP = hostip.String()
	}
	Info.Port = jetpack.Config().MustGetInt("mds.port")

	if s, err := hex.DecodeString(jetpack.Config().MustGet("mds.signing-key")); err != nil {
		panic(err)
	} else {
		SigningKey = s
	}

	switch lfPath, _ := jetpack.Config().Get("mds.logfile"); lfPath {
	case "-", "":
		log.SetOutput(os.Stderr)
	case "none", "/dev/null":
		log.SetOutput(ioutil.Discard)
	default:
		if lf, err := os.OpenFile(lfPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640); err != nil {
			log.Fatalf("Cannot open log file %#v: %v", lfPath, err)
		} else {
			log.SetOutput(lf)
			defer lf.Close()
		}
	}

	if pfPath, ok := jetpack.Config().Get("mds.pidfile"); ok {
		if err := ioutil.WriteFile(pfPath, []byte(fmt.Sprintln(os.Getpid())), 0644); err != nil {
			log.Fatalf("Cannot write pidfile %#v: %v", pfPath, err)
		}
	}

	addr := fmt.Sprintf("%v:%d", Info.IP, Info.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Cannot listen on %v: %v", addr, err)
	}

	if !jetpack.Config().GetBool("mds.keep-uid", false) {
		uid, gid := jetpack.MDSUidGid()
		if err := unix.Setgroups(nil); err != nil {
			log.Fatal("Cannot clear groups:", err)
		}
		if err := unix.Setresgid(gid, gid, gid); err != nil {
			log.Fatal("Cannot drop gid:", err)
		}
		if err := unix.Setresuid(uid, uid, uid); err != nil {
			log.Fatal("Cannot drop uid:", err)
		}
	}

	Info.Uid = os.Getuid()
	Info.Gid = os.Getgid()

	log.Println("Listening on:", addr)
	log.Fatal(http.Serve(listener, http.HandlerFunc(ServeMetadata)))
}
