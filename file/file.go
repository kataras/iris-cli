package file

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// Remote, fetched at `ListFiles`.
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
	// Local.
	Dest    string                 `json:"-"` // The destination path, including the filename (if does not contain a filename the "Name" will be used instead.)
	Package string                 `json:"-"` // The go package declaration.
	Data    map[string]interface{} `json:"-"` // Any template data.
	// Replacements map[string]string      `json:"-"` // Any replacements.
}

// Install downloads and performs necessary tasks to save a remote file.
func (f *File) Install() error {
	b, err := utils.Download(f.DownloadURL, nil)
	if err != nil {
		return err
	}

	fpath := utils.Dest(f.Dest)

	var newPkg []byte

	if f.Package != "" {
		newPkg = []byte(f.Package)
	} else {
		newPkg = utils.TryFindPackage(fpath)
	}

	if len(newPkg) > 0 {
		b = bytes.ReplaceAll(b, utils.Package(b), newPkg)
	}

	if isFile := utils.Ext(fpath) != ""; !isFile {
		fpath = filepath.Join(fpath, f.Name)
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
	return nil
}
