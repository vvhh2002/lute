// Lute - A structural markdown engine.
// Copyright (C) 2019, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package lute

// Tree is the representation of the markdown ast.
type Tree struct {
	Root      *Root
	name      string // the name of the input; used only for error reports
	text      string
	lex       *lexer
	token     [3]item
	peekCount int
}

func (t *Tree) HTML() string {
	return t.Root.HTML()
}

func Parse(name, text string) (*Tree, error) {
	t := &Tree{name: name, text: text}
	err := t.parse()

	return t, err
}

// next returns the next token.
func (t *Tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}

	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *Tree) backup() {
	t.peekCount++
}

// backup2 backs the input stream up two tokens.
// The zeroth token is already there.
func (t *Tree) backup2(t1 item) {
	t.token[1] = t1
	t.peekCount = 2
}

// backup3 backs the input stream up three tokens
// The zeroth token is already there.
func (t *Tree) backup3(t2, t1 item) {
	// Reverse order: we're pushing back.
	t.token[1] = t1
	t.token[2] = t2
	t.peekCount = 3
}

// peek returns but does not consume the next token.
func (t *Tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}

	t.peekCount = 1
	t.token[0] = t.lex.nextItem()

	return t.token[0]
}

// nextNonSpace returns the next non-space token.
func (t *Tree) nextNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}

	return token
}

// Parsing.

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Tree) recover(errp *error) {
	e := recover()
	if e != nil {
		if t != nil {
			t.lex.drain()
			t.stopParse()
		}
		*errp = e.(error)
	}
}

// startParse initializes the parser, using the lexer.
func (t *Tree) startParse(lex *lexer) {
	t.Root = nil
	t.lex = lex
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
}

func (t *Tree) parse() (err error) {
	defer t.recover(&err)
	t.startParse(lex(t.name, t.text))
	t.parseContent()
	t.stopParse()

	return nil
}

func (t *Tree) acceptSpaces() (ret int) {
	for {
		token := t.next()
		if itemSpace != token.typ {
			t.backup()

			break
		}
		ret++
	}
	if 4 <= ret {
		t.backup()
	}

	return
}

func (t *Tree) acceptTabs(tabs int) {
	for i := 0; i < tabs; i++ {
		token := t.next()
		if itemTab != token.typ {
			t.backup()

			break
		}
	}
}

func (t *Tree) parseContent() {
	t.Root = &Root{Parent{NodeType: NodeRoot, Pos: 0}}

	for token := t.peek(); itemEOF != token.typ && itemError != token.typ; token = t.peek() {
		var c Node
		switch token.typ {
		case itemSpace:
			spaces := t.acceptSpaces()
			if 4 <= spaces {
				c = t.parseCode()

				break
			}

			fallthrough
		case itemStr, itemParagraph, itemHeading, itemThematicBreak, itemQuote, itemListItem /* Table, HTML */, itemCode, // BlockContent
			itemTab:
			c = t.parseTopLevelContent()
		default:
			c = t.parsePhrasingContent()
		}

		t.Root.append(c)
	}
}

func (t *Tree) parseTopLevelContent() (ret Node) {
	ret = t.parseBlockContent(0)

	return
}

func (t *Tree) acceptIndent(indentLevel int) {
	if 1 > indentLevel {
		return
	}

	t.acceptTabs(indentLevel)
}

func (t *Tree) parseBlockContent(indentLevel int) Node {
	for {
		t.acceptTabs(indentLevel)

		switch token := t.peek(); token.typ {
		case itemParagraph:
			t.next() // consume \n\n
			continue
		case itemStr:
			return t.parseParagraph()
		case itemHeading:
			return t.parseHeading()
		case itemThematicBreak:
			return t.parseThematicBreak()
		case itemQuote:
			return t.parseBlockquote()
		case itemInlineCode:
			return t.parseInlineCode()
		case itemCode, itemTab:
			return t.parseCode()
		case itemListItem:
			return t.parseList()
		default:
			return nil
		}
	}
}

func (t *Tree) parseListContent() Node {

	return nil
}

func (t *Tree) parseTableContent() Node {

	return nil
}

func (t *Tree) parseRowContent() Node {

	return nil
}

func (t *Tree) parsePhrasingContent() (ret Node) {
	ret = t.parseStaticPhrasingContent()

	return
}

func (t *Tree) parseStaticPhrasingContent() (ret Node) {
	switch token := t.peek(); token.typ {
	case itemStr:
		return t.parseText()
	case itemEm:
		ret = t.parseEm()
	case itemStrong:
		ret = t.parseStrong()
	case itemInlineCode:
		ret = t.parseInlineCode()
	case itemBreak:
		ret = t.parseBreak()
	}

	return
}

func (t *Tree) parseParagraph() Node {
	token := t.peek()

	ret := &Paragraph{
		Parent{NodeParagraph, token.pos, nil},
		[]Node{},
	}

	for {
		c := t.parsePhrasingContent()
		if nil == c {
			ret.trim()

			break
		}

		ret.append(c)
	}

	return ret
}

func (t *Tree) parseHeading() (ret Node) {
	token := t.next()
	t.next() // consume spaces

	ret = &Heading{
		Parent{NodeHeading, token.pos, nil},
		len(token.val),
		[]Node{t.parsePhrasingContent()},
	}

	return
}

func (t *Tree) parseThematicBreak() (ret Node) {
	token := t.next()
	ret = &ThematicBreak{NodeThematicBreak, token.pos}

	return
}

func (t *Tree) parseBlockquote() (ret Node) {
	token := t.next()
	t.next() // consume spaces

	ret = &Blockquote{
		Parent{NodeParagraph, token.pos, nil},
		[]Node{t.parseBlockContent(0)},
	}

	return
}

func (t *Tree) parseText() Node {
	token := t.next()

	return &Text{Literal{NodeText, token.pos, token.val}}
}

func (t *Tree) parseEm() (ret Node) {
	t.next() // consume open *
	token := t.peek()
	ret = &Emphasis{
		Parent{NodeEmphasis, token.pos, nil},
		[]Node{t.parsePhrasingContent()},
	}
	t.next() // consume close *

	return
}

func (t *Tree) parseStrong() (ret Node) {
	t.next() // consume open **
	token := t.peek()
	ret = &Strong{
		Parent{NodeStrong, token.pos, nil},
		[]Node{t.parsePhrasingContent()},
	}
	t.next() // consume close **

	return
}

func (t *Tree) parseDelete() (ret Node) {
	t.next() // consume open ~~
	token := t.peek()
	ret = &Delete{
		Parent{NodeDelete, token.pos, nil},
		[]Node{t.parsePhrasingContent()},
	}
	t.next() // consume close ~~

	return
}

func (t *Tree) parseHTML() (ret Node) {
	return nil
}

func (t *Tree) parseBreak() (ret Node) {
	token := t.next()
	ret = &Break{NodeBreak, token.pos}

	return
}

func (t *Tree) parseInlineCode() (ret Node) {
	t.next() // consume open `

	code := t.next()
	ret = &InlineCode{Literal{NodeInlineCode, code.pos, code.val}}

	t.next() // consume close `

	return
}

func (t *Tree) parseCode() (ret Node) {
	t.next() // consume open ```

	token := t.next()
	pos := token.pos
	var code string
	for ; itemCode != token.typ && itemEOF != token.typ; token = t.next() {
		code += token.val
		if itemBreak == token.typ {
			if itemCode == t.peek().typ {
				break
			}

			spaces := t.acceptSpaces()
			if 4 > spaces {
				break
			}
		}
	}

	ret = &Code{Literal{NodeCode, pos, code}, "", ""}

	if itemEOF == t.peek().typ {
		return
	}

	t.next() // consume close ```

	return
}

func (t *Tree) parseList() Node {
	t.next() // *
	t.next() // space

	token := t.peek()
	list := &List{
		Parent:   Parent{NodeList, token.pos, nil},
		Ordered:  false,
		Start:    1,
		Spread:   false,
		Children: []Node{},
	}

	for {
		c := t.parseListItem()
		if nil == c {
			break
		}
		list.append(c)
	}

	return list
}

func (t *Tree) parseListItem() Node {
	token := t.peek()
	if itemEOF == token.typ {
		return nil
	}

	ret := &ListItem{
		Parent:   Parent{NodeListItem, token.pos, nil},
		Checked:  false,
		Spread:   false,
		Children: []Node{},
	}

	for {
		c := t.parseBlockContent(1)
		if nil == c {
			break
		}

		ret.append(c)
	}

	return ret
}

type stack struct {
	items []interface{}
	count int
}

func (s *stack) push(e interface{}) {
	s.items = append(s.items[:s.count], e)
	s.count++
}

func (s *stack) pop() interface{} {
	if s.count == 0 {
		return nil
	}

	s.count--

	return s.items[s.count]
}

func (s *stack) peek() interface{} {
	if s.count == 0 {
		return nil
	}

	return s.items[s.count-1]
}

func (s *stack) isEmpty() bool {
	return 0 == len(s.items)
}
