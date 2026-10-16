package bilara

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

type File struct {
	Stream model.Stream
	Muid   string
	Path   string
}

const (
	rootSuffix      = "_root-pli-ms.json"
	htmlSuffix      = "_html.json"
	referenceSuffix = "_reference.json"
)

var (
	translationNameRE = regexp.MustCompile(`^([a-z0-9][a-z0-9.-]*)_translation-en-[a-z0-9-]+\.json$`)
	fullKeyRE         = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*:[0-9]+(\.[0-9]+)+$`)
)

var streams = []model.Stream{model.StreamRoot, model.StreamTranslation, model.StreamHTML, model.StreamReference}

func MuidFromFilename(name string) (string, error) {
	for _, s := range streams {
		if muid, ok := matchName(s, name); ok {
			if muid == "" {
				return "", fmt.Errorf("%q: empty muid", name)
			}
			return muid, nil
		}
	}
	return "", fmt.Errorf("%q: not a bilara segment file name", name)
}

func matchName(stream model.Stream, name string) (string, bool) {
	if stream == model.StreamTranslation {
		m := translationNameRE.FindStringSubmatch(name)
		if m == nil {
			return "", false
		}
		return m[1], true
	}
	var suffix string
	switch stream {
	case model.StreamRoot:
		suffix = rootSuffix
	case model.StreamHTML:
		suffix = htmlSuffix
	case model.StreamReference:
		suffix = referenceSuffix
	default:
		return "", false
	}
	if !strings.HasSuffix(name, suffix) {
		return "", false
	}
	return strings.TrimSuffix(name, suffix), true
}

func ValidFullKey(k string) bool { return fullKeyRE.MatchString(k) }

func relSlash(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func Walk(root string, yield func(File) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		rel, err := relSlash(root, path)
		if err != nil {
			return err
		}
		top, _, _ := strings.Cut(rel, "/")
		stream := model.Stream(top)
		switch stream {
		case model.StreamRoot, model.StreamTranslation, model.StreamHTML, model.StreamReference:
		default:
			return nil
		}
		muid, ok := matchName(stream, filepath.Base(path))
		if !ok || muid == "" {
			return fmt.Errorf("bilara walk: %s: name does not match %s stream suffix", path, stream)
		}
		return yield(File{Stream: stream, Muid: muid, Path: path})
	})
}

func WalkNames(root string, yield func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		rel, err := relSlash(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, "/")
		if parts[0] != string(model.StreamTranslation) {
			return nil
		}
		for _, p := range parts[1:] {
			if p == "name" {
				return yield(path)
			}
		}
		return nil
	})
}
