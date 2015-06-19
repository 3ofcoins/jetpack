package pod_constructor

import (
	"reflect"
	"testing"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

func TestCPMFlags(t *testing.T) {
	// Three forms: separated, joined, joined with equal sign
	for _, sample := range [][]string{
		[]string{"-a", "foo=bar"},
		[]string{"-afoo=bar"},
		[]string{"-a=foo=bar"},
	} {
		if pm, err := ConstructPodManifest(nil, sample); err != nil {
			t.Fatal(err)
		} else if ann := pm.Annotations; len(ann) != 1 {
			t.Fatalf("Got %d annotations, expected 1", len(ann))
		} else if v, ok := ann.Get("foo"); !ok {
			t.Fatal("Annotation not set")
		} else if v != "bar" {
			t.Fatalf("Annotation set to %#v, expected \"bar\"", v)
		}
	}
}

func mustHash(s string) types.Hash {
	if hash, err := types.NewHash(s); err != nil {
		panic(err)
	} else {
		return *hash
	}
}

func TestCPM(t *testing.T) {
	// One big, complex test; TODO: flesh out simpler cases
	if pm, err := ConstructPodManifest(nil, []string{
		// Annotations
		"-afoo=bar",
		// Sane volumes
		"-vfoo", "-vbar:/tmp", "-v", "-baz:/etc",
		// Mix in some more annotations
		"-abaz=quux",
		// Rocket-style volumes
		"-vbarney,kind=empty",
		"-vfred,kind=host,source=/tmp",
		"-vbammbamm,kind=host,source=/etc,readOnly=true",
		// App with image specified by hash
		"first", "sha512-cafebabe",
		"-mfoo", "-mxyzzy:fred",
		// App with discovery string and explicitly set name
		"second", "example.com/whatever:1.2.3",
		"-afoo=bar",
		// Discovery string and implicit name
		"-", "example.com/third,os=freebsd",
	}); err != nil {
		t.Fatal(err)
	} else {
		if !reflect.DeepEqual(pm.Annotations, types.Annotations{
			types.Annotation{"foo", "bar"},
			types.Annotation{"baz", "quux"},
		}) {
			t.Fatalf("Unexpected annotations: %#v", pm.Annotations)
		}

		truth := true
		if !reflect.DeepEqual(pm.Volumes, []types.Volume{
			types.Volume{"foo", "empty", "", nil},
			types.Volume{"bar", "host", "/tmp", nil},
			types.Volume{"baz", "host", "/etc", &truth},
			types.Volume{"barney", "empty", "", nil},
			types.Volume{"fred", "host", "/tmp", nil},
			types.Volume{"bammbamm", "host", "/etc", &truth},
		}) {
			t.Fatalf("Unexpected volumes: %#v", pm.Volumes)
		}

		if expected := (schema.AppList{
			schema.RuntimeApp{
				Name: types.ACName("first"),
				Image: schema.RuntimeImage{
					ID: mustHash("sha512-cafebabe"),
				},
				Mounts: []schema.Mount{
					schema.Mount{"foo", "foo"},
					schema.Mount{"fred", "xyzzy"},
				},
			},
			schema.RuntimeApp{
				Name: types.ACName("second"),
				Image: schema.RuntimeImage{
					Name:   types.MustACName("example.com/whatever"),
					Labels: types.Labels{types.Label{"version", "1.2.3"}},
				},
				Annotations: types.Annotations{
					types.Annotation{"foo", "bar"},
				},
			},
			schema.RuntimeApp{
				Name: types.ACName("third"),
				Image: schema.RuntimeImage{
					Name:   types.MustACName("example.com/third"),
					Labels: types.Labels{types.Label{"os", "freebsd"}},
				},
			},
		}); !reflect.DeepEqual(pm.Apps, expected) {
			t.Fatalf("Got apps: %#v (expected: %#v)", pm.Apps, expected)
		}
	}
}
