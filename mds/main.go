package main

import "encoding/json"
import "flag"
import "fmt"
import "log"
import "net"
import "net/http"
import "os"
import "strings"

import "github.com/juju/errors"

import "../jetpack"

var Host *jetpack.Host

func getListenIP() (string, error) {
	ifname := Host.Properties.MustGetString("jail.interface")
	if iface, err := net.InterfaceByName(ifname); err != nil {
		return "", errors.Trace(err)
	} else {
		if addrs, err := iface.Addrs(); err != nil {
			return "", errors.Trace(err)
		} else {
			if ip, _, err := net.ParseCIDR(addrs[0].String()); err != nil {
				return "", errors.Trace(err)
			} else {
				return fmt.Sprintf("%v:%d", ip, Host.Properties.MustGetInt("mds.port")), nil
			}
		}
	}
}

func getPod(ip string) *jetpack.Pod {
	for _, pod := range Host.Pods() {
		if podIp, _ := pod.Manifest.Annotations.Get("ip-address"); podIp == ip {
			return pod
		}
	}
	return nil
}

func withPod(w http.ResponseWriter, r *http.Request, fn func(*jetpack.Pod)) {
	clientIP := strings.SplitN(r.RemoteAddr, ":", 2)[0]
	if hdr, ok := r.Header["Metadata-Flavor"]; !ok || len(hdr) != 1 || hdr[0] != "AppContainer" {
		log.Println("ERR: no Metadata-Flavor: header from", clientIP)
		w.WriteHeader(http.StatusBadRequest)
	} else if pod := getPod(clientIP); pod == nil {
		log.Println("ERR: No pod for IP %v", clientIP)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		log.Println("Req from:", clientIP, pod.UUID)
		fn(pod)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintf(w, "Jetpack metadata service version %v (%v) built on %v\n",
				jetpack.Version,
				jetpack.Revision,
				jetpack.BuildTimestamp)
		} else {
			http.NotFound(w, r)
		}
	})

	http.HandleFunc("/acMetadata/v1/pod/uuid", func(w http.ResponseWriter, r *http.Request) {
		withPod(w, r, func(pod *jetpack.Pod) {
			w.Write([]byte(pod.UUID.String()))
		})
	})

	http.HandleFunc("/acMetadata/v1/pod/manifest", func(w http.ResponseWriter, r *http.Request) {
		withPod(w, r, func(pod *jetpack.Pod) {
			if manifestJSON, err := json.Marshal(pod.Manifest); err != nil {
				panic(err)
			} else {
				w.Write(manifestJSON)
			}
		})
	})

	http.HandleFunc("/acMetadata/v1/pod/annotations/", func(w http.ResponseWriter, r *http.Request) {
		withPod(w, r, func(pod *jetpack.Pod) {
			annName := r.URL.Path[len("/acMetadata/v1/pod/annotations/"):]
			if val, ok := pod.Manifest.Annotations.Get(annName); ok {
				w.Write([]byte(val))
			} else {
				http.NotFound(w, r)
			}
		})
	})

	http.HandleFunc("/acMetadata/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
		withPod(w, r, func(pod *jetpack.Pod) {
			subpath := r.URL.Path[len("/acMetadata/v1/apps/"):]
			// ALL VALID RESPONSES NEED TO RETURN. If no valid path is found
			// for one app, we continue iteration, because one app's name
			// may be a prefix of other app's name within same pod. We
			// return 404 only if we can't find a valid URL for any of the
			// apps, after the loop.
			for _, app := range pod.Manifest.Apps {
				appPrefix := string(app.Name) + "/"
				if strings.HasPrefix(subpath, appPrefix) {
					switch appPath := subpath[len(appPrefix):]; appPath {
					case "image/id":
						w.Write([]byte(app.Image.ID.String()))
						return
					case "image/manifest":
						fmt.Fprintln(w, "FIXME")
						return
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
		})
	})

	if addr, err := getListenIP(); err != nil {
		log.Fatalln("Error getting listen IP:", err)
	} else {
		log.Println("Listening on:", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}
