package snippet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/kataras/iris-cli/utils"
)

const supportedType = "file" // ignore dirs.

// ListFiles returns a github repository's files.
func ListFiles(repo, version string) ([]*File, error) {
	repo, version = utils.SplitNameVersion(repo)
	if version == "" {
		version = "master"
	}

	var resp []*File

	url := fmt.Sprintf("https://api.github.com/repos/%s/contents?ref=%s", repo, version)
	b, err := utils.Download(url, nil)
	if err != nil {
		return nil, err
	}

	var files []*File

	if err := json.Unmarshal(b, &files); err != nil {
		return nil, err
	}

	for _, f := range resp {
		if f.Type != supportedType || f.DownloadURL == "" || f.DownloadURL == "null" {
			continue
		}

		files = append(files, f)
	}

	return files, nil
}

// File represents a github file to be locally saved.
// See `ListFiles` package-level function too.
type File struct {
	// Remote, should be filled if direct.
	Repo string `json:"-"`
	// Remote, fetched at `ListFiles`.
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
	Version     string `json:"-"`
	// Local.
	Dest         string                 `json:"-"` // The destination path, including the filename (if does not contain a filename the "Name" will be used instead.)
	Package      string                 `json:"-"` // The go package declaration.
	Data         map[string]interface{} `json:"-"` // Any template data.
	Replacements map[string]string      `json:"-"` // Any replacements.
}

// Install downloads and performs necessary tasks to save a remote file.
func (f *File) Install() error {
	if f.Version == "" || f.Version == "latest" {
		f.Version = "master"
	}

	if f.Repo == "" && f.DownloadURL != "" {
		parts := strings.Split(strings.TrimPrefix(f.DownloadURL, "https://"), "/")
		f.Repo = parts[1] + "/" + parts[2]
	}

	if !strings.Contains(f.DownloadURL, f.Version) {
		// direct single-file installation without ListFiles before or version changed.
		f.DownloadURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", f.Repo, f.Version, f.Name)
	}

	b, err := utils.Download(f.DownloadURL, nil)
	if err != nil {
		return err
	}

	fpath := utils.Dest(f.Dest)
	if isFile := utils.Ext(fpath) != ""; !isFile {
		fpath = filepath.Join(fpath, filepath.Base(f.Name))
	}

	var newPkg []byte

	if f.Package != "" {
		newPkg = []byte(f.Package)
	} else {
		newPkg = utils.TryFindPackage(fpath)
	}

	if len(newPkg) > 0 {
		b = bytes.ReplaceAll(b, utils.Package(b), newPkg)
	}

	if len(f.Replacements) > 0 {
		s := string(b)
		for oldValue, newValue := range f.Replacements {
			s = strings.ReplaceAll(s, oldValue, newValue)
		}

		b = []byte(s)
	}

	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if f.Data != nil {
		tmpl, err := template.New("").Parse(string(b))
		if err != nil {
			return err
		}
		err = tmpl.Execute(outFile, f.Data)
	} else {
		_, err = outFile.Write(b)
	}

	if err != nil {
		return err
	}

	f.Dest = fpath
	f.Package = string(newPkg)
	return nil
}
