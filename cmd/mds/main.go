package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net"

	"golang.org/x/sys/unix"

	"code.google.com/p/go-uuid/uuid"
)
import "flag"
import "fmt"
import "log"
import "net/http"
import "net/url"
import "os"
import "strings"
import "time"

import "github.com/3ofcoins/jetpack/lib/jetpack"

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

func resp200(v interface{}) (int, []byte) {
	return http.StatusOK, []byte(fmt.Sprint(v))
}

func resp404() (int, []byte) {
	return http.StatusNotFound, nil
}

func resp403() (int, []byte) {
	return http.StatusForbidden, nil
}

func resp500(err error) (int, []byte) {
	return http.StatusInternalServerError, []byte(err.Error())
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

func doServeMetadata(r *http.Request) (int, []byte) {
	if r.URL.Path == "/" {
		// Root URL. We introduce ourselves, no questions asked.
		return http.StatusOK, []byte(fmt.Sprintf("Jetpack metadata service version %v\n", jetpack.Version()))
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
			return http.StatusOK, body
		}
	}

	// All other requests should be coming from a pod.
	pod := getPod(clientIP(r))
	if pod == nil {
		return http.StatusTeapot, []byte("You are not a real pod. For you, I am a teapot.")
	}

	// hack hack hack
	r.URL.User = url.User(pod.UUID.String())

	if !jetpack.VerifyMetadataToken(pod.UUID, token) {
		return http.StatusTeapot, []byte("You are not a real pod. For you, I am a teapot.")
	}

	if !strings.HasPrefix(path, "/acMetadata/v1/") {
		// Not a metadata service request.
		return resp404()
	}

	path = path[len("/acMetadata/v1/"):]
	switch {

	case path == "pod/uuid":
		return resp200(pod.UUID)

	case path == "pod/manifest":
		// Pod manifest
		if manifestJSON, err := json.Marshal(pod.Manifest); err != nil {
			panic(err)
		} else {
			return http.StatusOK, manifestJSON
		}

	case path == "pod/hmac/sign":
		content := r.FormValue("content")
		if content == "" {
			return http.StatusBadRequest, []byte("content form value not found\n")
		}
		h := hmac.New(sha512.New, SigningKey)
		h.Write(pod.UUID)
		h.Write([]byte(content))
		return resp200(hex.EncodeToString(h.Sum(nil)))

	case path == "pod/hmac/verify":
		uuid := uuid.Parse(r.FormValue("uuid"))
		if uuid == nil {
			return http.StatusBadRequest, []byte(fmt.Sprintf("Invalid UUID: %#v\n", r.FormValue("uuid")))
		}

		sig, err := hex.DecodeString(r.FormValue("signature"))
		if err != nil {
			return http.StatusBadRequest, []byte(fmt.Sprintf("Invalid signature: %#v\n", r.FormValue("signature")))
		}

		content := r.FormValue("content")
		if content == "" {
			return http.StatusBadRequest, []byte("content form value not found\n")
		}

		h := hmac.New(sha512.New, SigningKey)
		h.Write(uuid)
		h.Write([]byte(content))

		if hmac.Equal(sig, h.Sum(nil)) {
			return http.StatusOK, nil
		} else {
			return http.StatusForbidden, nil
		}

	case path == "pod/annotations" || path == "pod/annotations/":
		anns := make([]string, len(pod.Manifest.Annotations))
		for i, ann := range pod.Manifest.Annotations {
			anns[i] = string(ann.Name)
		}
		return resp200(strings.Join(anns, "\n"))

	case strings.HasPrefix(path, "pod/annotations/"):
		// Pod annotation. 404 on nonexistent one.
		annName := path[len("pod/annotations/"):]
		if val, ok := pod.Manifest.Annotations.Get(annName); ok {
			return resp200(val)
		} else {
			return resp404()
		}
	case strings.HasPrefix(path, "apps/"):
		// App metadata.
		subpath := path[len("apps/"):]

		for _, app := range pod.Manifest.Apps {
			appPrefix := string(app.Name) + "/"
			if strings.HasPrefix(subpath, appPrefix) {
				switch appPath := subpath[len(appPrefix):]; appPath {

				case "image/id":
					return resp200(app.Image.ID)

				case "image/manifest":
					if img, err := Host.GetImage(app.Image.ID, "", nil); err != nil {
						panic(err)
					} else if manifestJSON, err := json.Marshal(img.Manifest); err != nil {
						panic(err)
					} else {
						return http.StatusOK, manifestJSON
					}

				case "annotations", "annotations/":
					anns := make([]string, len(app.Annotations))
					for i, ann := range app.Annotations {
						anns[i] = string(ann.Name)
					}
					return resp200(strings.Join(anns, "\n"))

				default:
					if strings.HasPrefix(appPath, "annotations/") {
						annName := appPath[len("annotations/"):]
						if val, ok := app.Annotations.Get(annName); ok {
							return resp200(val)
						} else if img, err := Host.GetImage(app.Image.ID, "", nil); err != nil {
							panic(err)
						} else if val, ok := img.Manifest.Annotations.Get(annName); ok {
							return resp200(val)
						}
					}
				}
			}
		}
		return resp404()

	default:
		return resp404()
	}
}

func ServeMetadata(w http.ResponseWriter, r *http.Request) {
	status, body := doServeMetadata(r)

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
