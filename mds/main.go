package main

import "encoding/json"
import "flag"
import "fmt"
import "log"
import "net/http"
import "net/url"
import "os"
import "strings"
import "time"

import "../jetpack"

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
	return http.StatusOK, []byte(fmt.Sprintf("%v", v))
}

func resp404() (int, []byte) {
	return http.StatusNotFound, nil
}

func resp500(err error) (int, []byte) {
	return http.StatusInternalServerError, []byte(err.Error())
}

func doServeMetadata(r *http.Request) (int, []byte) {
	if r.URL.Path == "/" {
		// Root URL. We introduce ourselves, no questions asked.
		return http.StatusOK, []byte(fmt.Sprintf(
			"Jetpack metadata service version %v (%v) built on %v\n",
			jetpack.Version,
			jetpack.Revision,
			jetpack.BuildTimestamp))
	}

	if !strings.HasPrefix(r.URL.Path, "/acMetadata/v1/") {
		// Not a metadata service request.
		return resp404()
	}

	// API request. Ensure it has required `Metadata-Flavor:
	// AppContainter' header and it comes from a container's IP.

	if hdr, ok := r.Header["Metadata-Flavor"]; !ok || len(hdr) != 1 || hdr[0] != "AppContainer" {
		return http.StatusBadRequest, []byte("Metadata-Flavor header missing or invalid")
	}

	pod := getPod(clientIP(r))
	if pod == nil {
		return http.StatusTeapot, []byte("You are not a pod. For you, I am a teapot.")
	}

	// hack hack hack
	r.URL.User = url.User(pod.UUID.String())

	path := r.URL.Path[len("/acMetadata/v1/"):]
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

	case path == "pod/hmac/sign" || path == "pod/hmac/verify":
		// HMAC sign/verify service.
		return http.StatusNotImplemented, nil

	case strings.HasPrefix(path, "pod/annotations/"):
		// Pod annotation. 404 on nonexistent one.
		annName := r.URL.Path[len("pod/annotations/"):]
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
					if img, err := Host.GetImageByHash(app.Image.ID); err != nil {
						panic(err)
					} else if manifestJSON, err := json.Marshal(img.Manifest); err != nil {
						panic(err)
					} else {
						return http.StatusOK, manifestJSON
					}

				default:
					if strings.HasPrefix(appPath, "annotations/") {
						if val, ok := app.Annotations.Get(appPath[len("annotations/"):]); ok {
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
		body = []byte(http.StatusText(status))
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

func main() {
	configPath := jetpack.DefaultConfigPath
	help := false

	if cfg := os.Getenv("JETPACK_CONF"); cfg != "" {
		configPath = cfg
	}

	flag.StringVar(&configPath, "config", configPath, "Configuration file")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&help, "help", false, "Show help")

	flag.Parse()
	// args := flag.Args()

	if host, err := jetpack.NewHost(configPath); err != nil {
		log.Fatalln("Error initializing host:", err)
	} else {
		Host = host
	}

	hostip, _, err := Host.HostIP()
	if err != nil {
		log.Fatalln("Error getting listen IP:", err)
	}

	addr := fmt.Sprintf("%v:%d", hostip, Host.Properties.MustGetInt("mds.port"))
	log.Println("Listening on:", addr)
	log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(ServeMetadata)))
}
