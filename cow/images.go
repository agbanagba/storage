package cow

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stringid"
)

var ErrImageUnknown = errors.New("image not known")

// Image is a record of a layer and an associated manifest.
// ID is either one specified at import-time or a randomly-generated value.
// Name is an optional user-defined convenience value.
type Image struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	TopLayer string `json:"layer"`
	Manifest string `json:"manifest,omitempty"`
}

type ImageStore interface {
	Create(id, name, layer, manifest string) (*Image, error)
	Exists(id string) bool
	Get(id string) (*Image, error)
	Delete(id string) error
	Wipe() error
	Images() ([]Image, error)
}

type imageStore struct {
	dir    string
	images []Image
	byid   map[string]*Image
	byname map[string]*Image
}

func (r *imageStore) Images() ([]Image, error) {
	return r.images, nil
}

func (r *imageStore) Load() error {
	rpath := filepath.Join(r.dir, "images.json")
	data, err := ioutil.ReadFile(rpath)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else {
		images := []Image{}
		ids := make(map[string]*Image)
		names := make(map[string]*Image)
		if err = json.Unmarshal(data, &images); len(data) == 0 || err == nil {
			for n, image := range images {
				ids[image.ID] = &images[n]
				if image.Name != "" {
					names[image.Name] = &images[n]
				}
			}
		}
		r.images = images
		r.byid = ids
		r.byname = names
	}
	return nil
}

func (r *imageStore) Save() error {
	rpath := filepath.Join(r.dir, "images.json")
	jdata, err := json.Marshal(&r.images)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(rpath, jdata, 0600)
}

func newImageStore(dir string) (ImageStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	istore := imageStore{
		dir:    dir,
		images: []Image{},
		byid:   make(map[string]*Image),
		byname: make(map[string]*Image),
	}
	if err := istore.Load(); err != nil {
		return nil, err
	}
	return &istore, nil
}

func (r *imageStore) Create(id, name, layer, manifest string) (image *Image, err error) {
	if id == "" {
		id = stringid.GenerateRandomID()
	}
	if err == nil {
		newImage := Image{
			ID:       id,
			Name:     name,
			TopLayer: layer,
			Manifest: manifest,
		}
		r.images = append(r.images, newImage)
		image = &r.images[len(r.images)-1]
		r.byid[id] = image
		if name != "" {
			r.byname[name] = image
		}
		err = r.Save()
	}
	return image, err
}

func (r *imageStore) Delete(id string) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if _, ok := r.byid[id]; ok {
		newImages := []Image{}
		for _, candidate := range r.images {
			if candidate.ID != id {
				newImages = append(newImages, candidate)
			}
		}
		r.images = newImages
		if err := r.Save(); err != nil {
			return err
		}
	}
	return nil
}

func (r *imageStore) Get(id string) (*Image, error) {
	if image, ok := r.byname[id]; ok {
		return image, nil
	}
	if image, ok := r.byid[id]; ok {
		return image, nil
	}
	return nil, ErrImageUnknown
}

func (r *imageStore) Exists(id string) bool {
	if _, ok := r.byname[id]; ok {
		return true
	}
	if _, ok := r.byid[id]; ok {
		return true
	}
	return false
}

func (r *imageStore) Wipe() error {
	ids := []string{}
	for id, _ := range r.byid {
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := r.Delete(id); err != nil {
			return err
		}
	}
	return nil
}
