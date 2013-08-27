// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parser implements a parser for Go source files. Input may be
// provided in a variety of forms; the output is an abstract syntax tree (AST)
// representing the Go source.
package parser

import (
	"bytes"
	"fmt"
	"go/token"
	"io"
	"io/ioutil"

	"github.com/cznic/scanner/go"
)

// ParseFile parses the source code of a single Go source file and returns the
// corresponding AST. The source code may be provided via the filename of the
// source file, or via the src parameter.
//
// If src != nil, ParseFile parses the source from src and the filename is only
// used when recording position information. The type of the argument for the
// src parameter must be string, []byte, or io.Reader. If src == nil, ParseFile
// parses the file specified by filename.
//
// The mode parameter controls the amount of source text parsed and other
// optional parser functionality.
//
// If the source couldn't be read, the returned AST is nil and the error
// indicates the specific failure.
func ParseFile(filename string, src interface{}) (f interface{}, err error) {
	var bsrc []byte
	switch x := src.(type) {
	case nil:
		if bsrc, err = ioutil.ReadFile(filename); err != nil {
			return
		}
	case string:
		bsrc = []byte(x)
	case []byte:
		bsrc = x
	case io.Reader:
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, x); err != nil {
			return nil, err
		}

		bsrc = buf.Bytes()
	}

	lx := lx{
		Scanner: scanner.New(bsrc),
	}
	lx.Scanner.Fname = filename
	if yyParse(&lx) != 0 || len(lx.Errors) != 0 {
		err = lx.Errors[0]
		//dbg("%v", err)
	}
	return
}

const COLAS = -1

var xlat = []int{
	//token.ILLEGAL:        ILLEGAL,
	token.EOF: 0,
	//token.COMMENT:        COMMENT,
	token.IDENT:          IDENTIFIER,
	token.INT:            INT_LIT,
	token.FLOAT:          FLOAT_LIT,
	token.IMAG:           IMAGINARY_LIT,
	token.CHAR:           RUNE_LIT,
	token.STRING:         STRING_LIT,
	token.ADD:            '+',
	token.SUB:            '-',
	token.MUL:            '*',
	token.QUO:            '/',
	token.REM:            '%',
	token.AND:            '&',
	token.OR:             '|',
	token.XOR:            '^',
	token.SHL:            LSH,
	token.SHR:            RSH,
	token.AND_NOT:        ANDNOT,
	token.ADD_ASSIGN:     ASSIGN_OP, // ADD_ASSIGN,
	token.SUB_ASSIGN:     ASSIGN_OP, // SUB_ASSIGN,
	token.MUL_ASSIGN:     ASSIGN_OP, // MUL_ASSIGN,
	token.QUO_ASSIGN:     ASSIGN_OP, // QUO_ASSIGN,
	token.REM_ASSIGN:     ASSIGN_OP, // REM_ASSIGN,
	token.AND_ASSIGN:     ASSIGN_OP, // AND_ASSIGN,
	token.OR_ASSIGN:      ASSIGN_OP, // OR_ASSIGN,
	token.XOR_ASSIGN:     ASSIGN_OP, // XOR_ASSIGN,
	token.SHL_ASSIGN:     ASSIGN_OP, // SHL_ASSIGN,
	token.SHR_ASSIGN:     ASSIGN_OP, // SHR_ASSIGN,
	token.AND_NOT_ASSIGN: ASSIGN_OP, // AND_NOT_ASSIGN,
	token.LAND:           ANDAND,
	token.LOR:            OROR,
	token.ARROW:          COMM,
	token.INC:            INC,
	token.DEC:            DEC,
	token.EQL:            EQ,
	token.LSS:            '<',
	token.GTR:            '>',
	token.ASSIGN:         '=',
	token.NOT:            '!',
	token.NEQ:            NE,
	token.LEQ:            LE,
	token.GEQ:            GE,
	token.DEFINE:         COLAS,
	token.ELLIPSIS:       DDD,
	token.LPAREN:         '(',
	token.LBRACK:         '[',
	token.LBRACE:         '{',
	token.COMMA:          ',',
	token.PERIOD:         '.',
	token.RPAREN:         ')',
	token.RBRACK:         ']',
	token.RBRACE:         '}',
	token.SEMICOLON:      ';',
	token.COLON:          ':',
	token.BREAK:          BREAK,
	token.CASE:           CASE,
	token.CHAN:           CHAN,
	token.CONST:          CONST,
	token.CONTINUE:       CONTINUE,
	token.DEFAULT:        DEFAULT,
	token.DEFER:          DEFER,
	token.ELSE:           ELSE,
	token.FALLTHROUGH:    FALLTHROUGH,
	token.FOR:            FOR,
	token.FUNC:           FUNC,
	token.GO:             GO,
	token.GOTO:           GOTO,
	token.IF:             IF,
	token.IMPORT:         IMPORT,
	token.INTERFACE:      INTERFACE,
	token.MAP:            MAP,
	token.PACKAGE:        PACKAGE,
	token.RANGE:          RANGE,
	token.RETURN:         RETURN,
	token.SELECT:         SELECT,
	token.STRUCT:         STRUCT,
	token.SWITCH:         SWITCH,
	token.TYPE:           TYPE,
	token.VAR:            VAR,
}

//TODO idlist_colas	= . // identifier { "," identifier } colas .
//TODO identifier_list = . // Manually enabled in proper contexts.
//TODO lbr = .

const (
	st1 = iota
	st2
	st3
	st4
	st5
	st6
	st7
	st8
	st9
	st10
	st11
	//st12
	st13
	st14
	st15
)

type pos struct {
	line, col int
}

type tok struct {
	tk  int
	val interface{}
	pos pos
}

type lx struct {
	*scanner.Scanner
	state     int
	dump      []tok
	toks      []tok
	ids       []tok
	preamble  int
	prev      tok
	prevValid bool
}

/*
_______________________________________________________________________________
(11:20) jnml@fsc-r550:~/src/github.com/cznic/parser/go$ cat fsm
const	C
struct  S
var	V
ident	I
colas	A
func	F

%%

{ident}?({const}|{var})\(?{ident}(,{ident})*	// identifier_list
{ident}?{struct}\{{ident}(,{ident})*		// identifier_list
{ident}?{func}{ident}?\({ident}(,{ident})*	// identifier_list
{ident}(,{ident})*{colas}			// idlist_colas

%%

PrimaryExpr:
|	FUNC Function
SourceFile2:
|	SourceFile2 FUNC IDENTIFIER Function ';'
|	SourceFile2 FUNC IDENTIFIER Signature ';'
|	SourceFile2 FUNC Receiver MethodName Function ';'
|	SourceFile2 FUNC Receiver MethodName Signature ';'
TypeLit:
|	FUNC Signature

----

FUNC '(' ... 
FUNC IDENTIFIER '(' ...
FUNC IDENTIFIER '(' ...
//TODO FUNC Receiver IDENTIFIER '(' ...
//TODO FUNC Receiver IDENTIFIER '(' ...
FUNC '(' ...
_______________________________________________________________________________
(11:20) jnml@fsc-r550:~/src/github.com/cznic/parser/go$ golex -DFA fsm
StartConditions:
	INITIAL, scId:0, stateId:1
DFA:
[1]
	"C", "V", --> 2
	"F"--> 5
	"I"--> 9
	"S"--> 13
[2]
	"("--> 3
	"I"--> 4
[3]
	"I"--> 4
[4]
	","--> 3
[5]
	"("--> 6
	"I"--> 8
[6]
	"I"--> 7
[7]
	","--> 6
[8]
	"("--> 6
[9]
	"C", "V", --> 2
	"F"--> 5
	","--> 10
	"A"--> 12
	"S"--> 13
[10]
	"I"--> 11
[11]
	","--> 10
	"A"--> 12
[12]
[13]
	"{"--> 14
[14]
	"I"--> 15
[15]
	","--> 14
state 4 accepts rule 1
state 7 accepts rule 3
state 12 accepts rule 4
state 15 accepts rule 2

_______________________________________________________________________________
(11:20) jnml@fsc-r550:~/src/github.com/cznic/parser/go$ 

*/

func (x *lx) Lex(lval *yySymType) (r int) {
	dbg("\n<<<< Lex state st%d", x.state+1)
	defer func() {
		var s string
		if r < 128 {
			s = string(r)
		} else {
			s = yyToknames[r-ANDAND]
		}
		dbg(">>>> %d:%d: returning %q, (%v)\n", lval.pos.line, lval.pos.col, s, lval.val)
	}()

dump:
	if len(x.dump) != 0 {
		tk := x.dump[0]
		r, lval.pos, lval.val = tk.tk, tk.pos, tk.val
		x.dump = x.dump[1:]
		if len(x.dump) == 0 {
			x.dump = nil
		}
		return
	}

	for {
		dbg("[state st%d]", x.state+1)
		tk := x.lex()

		switch r = tk.tk; x.state {
		case st1:
			switch r {
			case CONST, VAR:
				x.toks, x.state = []tok{tk}, st2
			case FUNC:
				panic("st1 func")
			case IDENTIFIER:
				x.toks, x.ids, x.state = []tok{tk}, []tok{tk}, st9
			case STRUCT:
				panic("st1 struct")
			default:
				lval.val, lval.pos = tk.val, tk.pos
				return
			}
		case st2:
			switch r {
			case '(':
				panic("st2 (")
			case IDENTIFIER:
				x.preamble = len(x.toks)
				x.toks, x.ids, x.state = append(x.toks, tk), []tok{tk}, st4
			default:
				panic("st2 default")
			}
		case st3:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st4:
			switch r {
			case ',':
				panic("st4 ,")
			default:
				x.dump, x.state = append(x.toks[:x.preamble], tok{IDENTIFIER_LIST, x.ids, x.ids[0].pos}, tk), st1
				goto dump
			}
		case st5:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st6:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st7:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st8:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st9:
			switch r {
			case CONST, VAR:
				panic("st9 const var")
			case FUNC:
				panic("st9 func")
			case ',':
				panic("st9 ,")
			case COLAS:
				panic("st9 :=")
			case STRUCT:
				panic("st9 struct")
			default:
				x.dump, x.state = append(x.toks, tk), st1
				goto dump
			}
		case st10:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st11:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st13:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st14:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		case st15:
			panic(fmt.Sprintf("TODO st%d", x.state+1))
		default:
			panic(fmt.Sprintf("internal error st%d", x.state+1))
		}
	}
}

func (x *lx) lex() (y tok) {
	defer func() {
		var s string
		if y.tk < 128 {
			s = string(y.tk)
		} else {
			s = yyToknames[y.tk-ANDAND]
		}
		dbg("........ returning %q", s)
	}()
	for {
		t, val := x.ScanSemis()
		if t == token.COMMENT {
			continue
		}

		tok := tok{xlat[t], val, pos{x.Line, x.Col}}
		//dbg("ScanSemis %v", t)
		if !x.prevValid {
			x.prev, x.prevValid = tok, true
			continue
		}

		if p, n := x.prev.tk, tok.tk; (p == ',' || p == ';') && (n == ')' || n == '}') {
			tok.val, x.prevValid = x.prev, false
			return tok
		}

		y, x.prev = x.prev, tok
		return
	}
}

func (x *lx) err(pos pos, format string, arg ...interface{}) {
	x.Error(fmt.Sprintf("%s:%d:%d: "+format, append([]interface{}{x.Fname, pos.line, pos.col}, arg...)...))
}

func (x *lx) error(format string, arg ...interface{}) {
	x.err(pos{x.Line, x.Col}, format, arg...)
}
