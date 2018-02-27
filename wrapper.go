package fproto_gowrap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RangelReale/fproto/fdep"
)

type Wrapper struct {
	dep *fdep.Dep

	PkgSource PkgSource
}

func NewWrapper(dep *fdep.Dep) *Wrapper {
	return &Wrapper{
		dep: dep,
	}
}

func (wp *Wrapper) GenerateFile(filename string, w io.Writer) error {
	g, err := NewGenerator(wp.dep, filename)
	g.PkgSource = wp.PkgSource
	if err != nil {
		return err
	}

	err = g.Generate()
	if err != nil {
		return err
	}

	fmt.Printf("*** %s\n", g.Filename())

	_, err = w.Write([]byte(g.String()))
	return err
}

func (wp *Wrapper) GenerateFiles(outputpath string) error {
	for _, df := range wp.dep.Files {
		if df.DepType == fdep.DepType_Own {
			g, err := NewGenerator(wp.dep, df.FilePath)
			if err != nil {
				return err
			}
			g.PkgSource = wp.PkgSource

			err = g.Generate()
			if err != nil {
				return err
			}

			p := filepath.Join(outputpath, g.Filename())

			err = os.MkdirAll(filepath.Dir(p), os.ModePerm)
			if err != nil {
				return err
			}

			file, err := os.Create(p)
			if err != nil {
				return err
			}

			_, err = file.WriteString(g.String())
			file.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
