package edit

import (
	"strings"

	"github.com/elves/elvish/edit/ui"

	"github.com/elves/elvish/eval"
	"github.com/elves/elvish/parse"
	"github.com/elves/elvish/util"
)

type commandComplContext struct {
	complContextCommon
}

const quotingForEmptySeed = parse.Bareword

func findCommandComplContext(n parse.Node, ev pureEvaler) complContext {
	// Determine if we are starting a new command. There are 3 cases:
	// 1. The whole chunk is empty (nothing entered at all): the leaf is a
	//    Chunk.
	// 2. Just after a newline or semicolon: the leaf is a Sep and its parent is
	//    a Chunk.
	// 3. Just after a pipe: the leaf is a Sep and its parent is a Pipeline.
	if parse.IsChunk(n) {
		return &commandComplContext{
			complContextCommon{"", parse.Bareword, n.End(), n.End()}}
	}
	if parse.IsSep(n) {
		parent := n.Parent()
		switch {
		case parse.IsChunk(parent), parse.IsPipeline(parent):
			return &commandComplContext{
				complContextCommon{"", quotingForEmptySeed, n.End(), n.End()}}
		case parse.IsPrimary(parent):
			ptype := parent.(*parse.Primary).Type
			if ptype == parse.OutputCapture || ptype == parse.ExceptionCapture {
				return &commandComplContext{
					complContextCommon{"", quotingForEmptySeed, n.End(), n.End()}}
			}
		}
	}

	if primary, ok := n.(*parse.Primary); ok {
		if compound, seed := primaryInSimpleCompound(primary, ev); compound != nil {
			if form, ok := compound.Parent().(*parse.Form); ok {
				if form.Head == compound {
					return &commandComplContext{
						complContextCommon{seed, primary.Type, compound.Begin(), compound.End()}}
				}
			}
		}
	}
	return nil
}

func (*commandComplContext) name() string { return "command" }

func (ctx *commandComplContext) generate(ev *eval.Evaler, ch chan<- rawCandidate) error {
	return complFormHeadInner(ctx.seed, ev, ch)
}

func complFormHeadInner(head string, ev *eval.Evaler, rawCands chan<- rawCandidate) error {
	if util.DontSearch(head) {
		return complFilenameInner(head, true, rawCands)
	}

	got := func(s string) {
		rawCands <- plainCandidate(s)
	}
	for special := range eval.IsBuiltinSpecial {
		got(special)
	}
	explode, ns, _ := eval.ParseVariable(head)
	if !explode {
		ev.EachVariableInTop(ns, func(varname string) {
			if strings.HasSuffix(varname, eval.FnSuffix) {
				got(eval.MakeVariableName(false, ns, varname[:len(varname)-len(eval.FnSuffix)]))
			} else {
				name := eval.MakeVariableName(false, ns, varname)
				rawCands <- &complexCandidate{name, " = ", " = ", ui.Styles{}}
			}
		})
	}
	eval.EachExternal(func(command string) {
		got(command)
		if strings.HasPrefix(head, "e:") {
			got("e:" + command)
		}
	})
	// TODO Support non-module namespaces.
	for name := range ev.Global {
		if head != name && strings.HasSuffix(name, eval.NsSuffix) {
			got(name)
		}
	}
	for name := range ev.Builtin {
		if head != name && strings.HasSuffix(name, eval.NsSuffix) {
			got(name)
		}
	}
	return nil
}
