package fproto_gowrap

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/RangelReale/fproto"
	"github.com/RangelReale/fproto/fdep"
)

type Generator struct {
	dep     *fdep.Dep
	filedep *fdep.FileDep
	b_head  *Builder
	b_body  *Builder

	imports map[string]string

	PkgSource  PkgSource
	TypeConvs  []TypeConverter
	tc_default TypeConverter
}

func NewGenerator(dep *fdep.Dep, filename string) (*Generator, error) {
	filedep, ok := dep.Files[filename]
	if !ok {
		return nil, fmt.Errorf("File %s not found", filename)
	}

	return &Generator{
		dep:        dep,
		filedep:    filedep,
		b_head:     NewBuilder(),
		b_body:     NewBuilder(),
		imports:    make(map[string]string),
		tc_default: &TypeConverter_Default{},
	}, nil
}

func (g *Generator) Body() *Builder {
	return g.b_body
}

func (g *Generator) Generate() error {
	for _, message := range g.filedep.ProtoFile.Messages {
		structName := CamelCaseSlice(strings.Split(message.Name, "."))
		sourceAlias := g.FileDep(g.filedep, "", true)

		g.b_body.P("type ", structName, " struct {")
		g.b_body.In()
		for _, fld := range message.Fields {
			tc := g.getTypeConv(message, fld)
			if tc != nil {
				var err error
				_, err = tc.GenerateField(g, message, fld)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("No type converter found")
			}
		}
		g.b_body.Out()
		g.b_body.P("}")
		g.b_body.P()

		// Import
		g.b_body.P("func (m *", structName, ") Import(s *", sourceAlias, ".", structName, ") error {")
		g.b_body.In()

		for _, fld := range message.Fields {
			tc := g.getTypeConv(message, fld)
			if tc != nil {
				var err error
				_, err = tc.GenerateFieldImport(g, message, fld)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("No type converter found")
			}
		}

		g.b_body.P("return nil")

		g.b_body.Out()
		g.b_body.P("}")
		g.b_body.P()

		// Export
		g.b_body.P("func (m *", structName, ") Export() (*", sourceAlias, ".", structName, ", error) {")
		g.b_body.In()

		g.b_body.P("ret := &", sourceAlias, ".", structName, "{}")

		for _, fld := range message.Fields {
			tc := g.getTypeConv(message, fld)
			if tc != nil {
				var err error
				_, err = tc.GenerateFieldExport(g, message, fld)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("No type converter found")
			}
		}

		g.b_body.P("return ret, nil")

		g.b_body.Out()
		g.b_body.P("}")
		g.b_body.P()

	}

	return nil
}

func (g *Generator) getTypeConv(message *fproto.MessageElement, fld *fproto.FieldElement) TypeConverter {
	tp, scalar := g.GetDepType(fld)

	if !scalar {
		for _, tc := range g.TypeConvs {
			for _, src := range tc.GetSources() {
				if src.FilePath == tp.FileDep.FilePath && src.PackageName == tp.FileDep.ProtoFile.PackageName {
					return tc
				}
			}
		}
	}

	return g.tc_default
}

func (g *Generator) GetDepType(fld *fproto.FieldElement) (tp *fdep.DepType, isscalar bool) {
	// check if if scalar
	if _, ok := fproto.ParseScalarType(fld.Type); ok {
		isscalar = true
	} else {
		isscalar = false
		var err error

		tp, err = g.filedep.GetType(fld.Type)
		if err != nil {
			log.Fatal(err)
		}
	}

	return
}

func (g *Generator) GetType(fld *fproto.FieldElement, pbsource bool) (t string, tp *fdep.DepType, isscalar bool) {
	// check if if scalar
	if st, ok := fproto.ParseScalarType(fld.Type); ok {
		t = st.String()
		isscalar = true
	} else {
		isscalar = false
		var err error

		tp, err = g.filedep.GetType(fld.Type)
		if err != nil {
			log.Fatal(err)
		}

		if tp.FileDep.IsSame(g.filedep) {
			//_ = g.FileDep(tp.FileDep, tp.Alias)
			t = fmt.Sprintf("%s", tp.Name)
		} else {
			falias := g.FileDep(tp.FileDep, tp.Alias, pbsource)
			t = fmt.Sprintf("%s.%s", falias, tp.Name)
		}
	}

	return
}

func (g *Generator) Dep(imp string, defalias string) string {
	var alias string
	var ok bool
	if alias, ok = g.imports[imp]; ok {
		return alias
	}

	if defalias == "" {
		defalias = path.Base(imp)
	}

	defalias = strings.Replace(defalias, ".", "_", -1)

	alias = defalias
	aliasct := 0
	aliasok := false
	for !aliasok {
		aliasok = true

		for _, a := range g.imports {
			if a == alias {
				aliasct++
				alias = fmt.Sprintf("%s%d", defalias, aliasct)
				aliasok = false
			}
		}

		if aliasok {
			break
		}
	}

	g.imports[imp] = alias
	return alias
}

func (g *Generator) FileDep(filedep *fdep.FileDep, defalias string, pbsource bool) string {
	var p string
	if !pbsource && !filedep.IsSame(g.filedep) && filedep.DepType == fdep.DepType_Own {
		p = g.GoWrapPackage(filedep)
	} else {
		p = filedep.GoPackage()
	}
	return g.Dep(p, defalias)
}

func (g *Generator) String() string {
	p := baseName(g.GoWrapPackage(g.filedep))

	g.b_head.P("package ", p)
	g.b_head.P()
	for i, ia := range g.imports {
		g.b_head.P("import ", ia, ` "`, i, `"`)
	}
	g.b_head.P()

	return g.b_head.String() + g.b_body.String()
}

func (g *Generator) Filename() string {
	p := g.GoWrapPackage(g.filedep)
	return path.Join(p, strings.TrimSuffix(path.Base(g.filedep.FilePath), path.Ext(g.filedep.FilePath))+".gpb.go")
}

func (g *Generator) GoWrapPackage(filedep *fdep.FileDep) string {
	if g.PkgSource != nil {
		if p, ok := g.PkgSource.GetPkg(filedep); ok {
			return p
		}
	}

	for _, o := range filedep.ProtoFile.Options {
		if o.Name == "gowrap_package" {
			return o.Value
		}
	}
	for _, o := range filedep.ProtoFile.Options {
		if o.Name == "go_package" {
			return o.Value
		}
	}
	return path.Dir(filedep.FilePath)
}
