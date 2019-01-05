package build

import (
	"errors"
	"go/types"

	"github.com/mmcloughlin/avo"
	"github.com/mmcloughlin/avo/buildtags"
	"github.com/mmcloughlin/avo/gotypes"
	"github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
	"golang.org/x/tools/go/packages"
)

// Context maintains state for incrementally building an avo File.
type Context struct {
	pkg      *packages.Package
	file     *avo.File
	function *avo.Function
	global   *avo.Global
	errs     []error
	reg.Collection
}

// NewContext initializes an empty build Context.
func NewContext() *Context {
	return &Context{
		file:       avo.NewFile(),
		Collection: *reg.NewCollection(),
	}
}

// Package sets the package the generated file will belong to. Required to be able to reference types in the package.
func (c *Context) Package(path string) {
	cfg := &packages.Config{
		Mode: packages.LoadTypes,
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		c.AddError(err)
		return
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		for _, err := range pkg.Errors {
			c.AddError(err)
		}
		return
	}
	c.pkg = pkg
}

// Constraints sets build constraints for the file.
func (c *Context) Constraints(t buildtags.ConstraintsConvertable) {
	cs := t.ToConstraints()
	if err := cs.Validate(); err != nil {
		c.AddError(err)
		return
	}
	c.file.Constraints = cs
}

// Constraint appends a constraint to the file's build constraints.
func (c *Context) Constraint(t buildtags.ConstraintConvertable) {
	c.Constraints(append(c.file.Constraints, t.ToConstraint()))
}

// Constraint appends a constraint to the file's build constraints. The
// constraint to add is parsed from the given expression. The expression should
// look the same as the content following "// +build " in regular build
// constraint comments.
func (c *Context) ConstraintExpr(expr string) {
	constraint, err := buildtags.ParseConstraint(expr)
	if err != nil {
		c.AddError(err)
		return
	}
	c.Constraint(constraint)
}

// Function starts building a new function with the given name.
func (c *Context) Function(name string) {
	c.function = avo.NewFunction(name)
	c.file.AddSection(c.function)
}

// Doc sets documentation comment lines for the currently active function.
func (c *Context) Doc(lines ...string) {
	c.activefunc().Doc = lines
}

// Attributes sets function attributes for the currently active function.
func (c *Context) Attributes(a avo.Attribute) {
	c.activefunc().Attributes = a
}

// Signature sets the signature for the currently active function.
func (c *Context) Signature(s *gotypes.Signature) {
	c.activefunc().SetSignature(s)
}

// SignatureExpr parses the signature expression and sets it as the active function's signature.
func (c *Context) SignatureExpr(expr string) {
	s, err := gotypes.ParseSignatureInPackage(c.types(), expr)
	if err != nil {
		c.AddError(err)
		return
	}
	c.Signature(s)
}

func (c *Context) types() *types.Package {
	if c.pkg == nil {
		return nil
	}
	return c.pkg.Types
}

// AllocLocal allocates size bytes in the stack of the currently active function.
// Returns a reference to the base pointer for the newly allocated region.
func (c *Context) AllocLocal(size int) operand.Mem {
	return c.activefunc().AllocLocal(size)
}

// Instruction adds an instruction to the active function.
func (c *Context) Instruction(i *avo.Instruction) {
	c.activefunc().AddNode(i)
}

// Label adds a label to the active function.
func (c *Context) Label(l avo.Label) {
	c.activefunc().AddLabel(l)
}

func (c *Context) activefunc() *avo.Function {
	if c.function == nil {
		c.AddErrorMessage("no active function")
		return avo.NewFunction("")
	}
	return c.function
}

//go:generate avogen -output zinstructions.go build

func (c *Context) StaticGlobal(name string) operand.Mem {
	c.global = avo.NewStaticGlobal(name)
	c.file.AddSection(c.global)
	return c.global.Base()
}

func (c *Context) DataAttributes(a avo.Attribute) {
	c.activeglobal().Attributes = a
}

func (c *Context) AddDatum(offset int, v operand.Constant) {
	if err := c.activeglobal().AddDatum(avo.NewDatum(offset, v)); err != nil {
		c.AddError(err)
	}
}

func (c *Context) AppendDatum(v operand.Constant) {
	c.activeglobal().Append(v)
}

func (c *Context) activeglobal() *avo.Global {
	if c.global == nil {
		c.AddErrorMessage("no active global")
		return avo.NewStaticGlobal("")
	}
	return c.global
}

func (c *Context) AddError(err error) {
	e := exterr(err)
	c.errs = append(c.errs, e)
}

func (c *Context) AddErrorMessage(msg string) {
	c.AddError(errors.New(msg))
}

func (c *Context) Result() (*avo.File, []error) {
	return c.file, c.errs
}
