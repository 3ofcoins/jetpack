package main

import "encoding/json"
import "flag"
import "fmt"
import "log"
import "net/http"
import "os"
import "strings"

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

func ServeMetadata(w http.ResponseWriter, r *http.Request) {
	clientIP := strings.SplitN(r.RemoteAddr, ":", 2)[0]

	if r.URL.Path == "/" {
		// Root URL. We introduce ourselves, no questions asked.
		fmt.Fprintf(w, "Jetpack metadata service version %v (%v) built on %v\n",
			jetpack.Version,
			jetpack.Revision,
			jetpack.BuildTimestamp)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/acMetadata/v1/") {
		// Not a metadata service request.
		http.NotFound(w, r)
		return
	}

	// API request. Ensure it has required `Metadata-Flavor:
	// AppContainter' header and it comes from a container's IP.

	if hdr, ok := r.Header["Metadata-Flavor"]; !ok || len(hdr) != 1 || hdr[0] != "AppContainer" {
		log.Println("ERR: no Metadata-Flavor: header from", clientIP)
		http.Error(w, "Metadata-Flavor header missing or invalid", http.StatusBadRequest)
		return
	}

	pod := getPod(clientIP)
	if pod == nil {
		log.Println("ERR: No pod for IP %v", clientIP)
		http.Error(w, "Not a pod", http.StatusBadRequest)
		return
	}

	path := r.URL.Path[len("/acMetadata/v1/"):]
	log.Println(clientIP, pod.UUID, path)
	switch {

	case path == "pod/uuid":
		// Pod UUID
		w.Write([]byte(pod.UUID.String()))

	case path == "pod/manifest":
		// Pod manifest
		if manifestJSON, err := json.Marshal(pod.Manifest); err != nil {
			panic(err)
		} else {
			w.Write(manifestJSON)
		}

	case path == "pod/hmac/sign" || path == "pod/hmac/verify":
		// HMAC sign/verify service.
		http.Error(w, "Not implemented", http.StatusNotImplemented)

	case strings.HasPrefix(path, "pod/annotations/"):
		// Pod annotation. 404 on nonexistent one.
		annName := r.URL.Path[len("pod/annotations/"):]
		if val, ok := pod.Manifest.Annotations.Get(annName); ok {
			w.Write([]byte(val))
		} else {
			http.NotFound(w, r)
		}
	case strings.HasPrefix(path, "apps/"):
		// App metadata.
		subpath := path[len("apps/"):]

		// ALL VALID RESPONSES IN THE LOOP NEED TO RETURN. If no
		// valid path is found for one app, we continue iterating,
		// because one app's name may be a prefix of another app's
		// name within same pod. We return 404 only if we can't
		// find a valid URL for any of the apps, after the loop.
		for _, app := range pod.Manifest.Apps {
			appPrefix := string(app.Name) + "/"
			if strings.HasPrefix(subpath, appPrefix) {
				switch appPath := subpath[len(appPrefix):]; appPath {

				case "image/id":
					w.Write([]byte(app.Image.ID.String()))
					return

				case "image/manifest":
					if img, err := Host.GetImageByHash(app.Image.ID); err != nil {
						panic(err)
					} else if manifestJSON, err := json.Marshal(img.Manifest); err != nil {
						panic(err)
					} else {
						w.Write(manifestJSON)
						return
					}

				default:
					if strings.HasPrefix(appPath, "annotations/") {
						if val, ok := app.Annotations.Get(appPath[len("annotations/"):]); ok {
							w.Write([]byte(val))
							return
						}
					}
				}
			}
		}
		http.NotFound(w, r)

	default:
		http.NotFound(w, r)
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
