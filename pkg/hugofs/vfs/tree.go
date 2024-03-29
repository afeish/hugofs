// package vfs provides a simple ASCII tree composing tool.
package vfs

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/afeish/hugo/global"
)

var (
	EdgeTypeLink EdgeType = "│"
	EdgeTypeMid  EdgeType = "├──"
	EdgeTypeEnd  EdgeType = "└──"
)

// IndentSize is the number of spaces per tree level.
var IndentSize = 3

func (d *Dir) FindLastNode() Tree {
	ns := d.children()
	if len(ns) == 0 {
		return nil
	}
	tree := ns[len(ns)-1]
	return tree
}

func (d *Dir) AddFile(v Value) Tree {
	_n := &File{
		d:    d,
		leaf: v.(string),
	}

	d.items.Set(_n.leaf, _n)
	return d
}

func (d *Dir) AddMetaFile(meta MetaValue, v Value) Tree {
	_n := &Dir{
		Parent: d,
		path:   v.(string),
		items:  newItems(nil),
	}
	_n.SetSys(meta)
	d.items.Set(_n.path, _n)
	return d
}

func (d *Dir) AddDir(v Value) Tree {
	branch := &Dir{
		Parent: d,
		path:   v.(string),
		items:  newItems(nil),
	}
	d.items.Set(branch.path, branch)
	return branch
}

func (d *Dir) AddMetaDir(meta MetaValue, v Value) Tree {
	branch := &Dir{
		Parent: d,
		path:   v.(string),
		items:  newItems(nil),
	}
	branch.SetSys(meta)
	d.items.Set(branch.path, branch)
	return branch
}

func (d *Dir) Dir() Tree {
	d.Parent = nil
	return d
}

func (d *Dir) FindByMeta(meta MetaValue) (t Tree) {
	d.items.Visit(func(node Node) bool {
		if reflect.DeepEqual(node.Sys(), meta) {
			t = node
			return false
		}
		if v := node.FindByMeta(meta); v != nil {
			t = v
			return false
		}
		return true
	})
	return nil
}

func (d *Dir) SetValue(value Value) {
	d.path = value.(string)
}

func (d *Dir) FindByValue(value Value) (t Tree) {
	d.items.Visit(func(node Node) bool {
		if reflect.DeepEqual(node.Name(), value) {
			t = node
			return false
		}
		if v := node.FindByMeta(value); v != nil {
			t = v
			return false
		}
		return true
	})

	return
}

func (d *Dir) Bytes() []byte {
	buf := new(bytes.Buffer)
	level := 0
	var levelsEnded []int

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.Sys() != nil {
		buf.WriteString(fmt.Sprintf("[%v]  %v", d.Sys(), d.path))
	} else {
		buf.WriteString(fmt.Sprintf("%v", d.path))
	}
	buf.WriteByte('\n')

	if d.items.Len() > 0 {
		print(buf, level, levelsEnded, d.children())
	}
	return buf.Bytes()
}

func (d *Dir) ToPrint() string {
	return string(d.Bytes())
}

func (d *Dir) String() string {
	return d.Path()
}

func print(wr io.Writer,
	level int, levelsEnded []int, nodes []Node) {

	for i, node := range nodes {
		edge := EdgeTypeMid
		if i == len(nodes)-1 {
			levelsEnded = append(levelsEnded, level)
			edge = EdgeTypeEnd
		}
		printValues(wr, level, levelsEnded, edge, node)

		if node.IsDir() {
			children := node.(*Dir).children()
			print(wr, level+1, levelsEnded, children)
		}
	}
}

func printValues(wr io.Writer,
	level int, levelsEnded []int, edge EdgeType, node Node) {

	for i := 0; i < level; i++ {
		if isEnded(levelsEnded, i) {
			fmt.Fprint(wr, strings.Repeat(" ", IndentSize+1))
			continue
		}
		fmt.Fprintf(wr, "%s%s", EdgeTypeLink, strings.Repeat(" ", IndentSize))
	}

	val := renderValue(level, node)
	meta := node.Sys()

	if meta != nil {
		fmt.Fprintf(wr, "%s [%2v]  %v\n", edge, meta, val)
		return
	}
	fmt.Fprintf(wr, "%s %v\n", edge, val)
}

func isEnded(levelsEnded []int, level int) bool {
	for _, l := range levelsEnded {
		if l == level {
			return true
		}
	}
	return false
}

func renderValue(level int, node Node) Value {
	if global.IsNil(node) {
		panic("node is nil")
	}
	lines := strings.Split(fmt.Sprintf("%v", node.Name()), "\n")

	// If value does not contain multiple lines, return itself.
	if len(lines) < 2 {
		return node.Name()
	}

	// If value contains multiple lines,
	// generate a padding and prefix each line with it.
	pad := padding(level, node)

	for i := 1; i < len(lines); i++ {
		lines[i] = fmt.Sprintf("%s%s", pad, lines[i])
	}

	return strings.Join(lines, "\n")
}

// padding returns a padding for the multiline values with correctly placed link edges.
// It is generated by traversing the tree upwards (from leaf to the root of the tree)
// and, on each level, checking if the Node the last one of its siblings.
// If a Node is the last one, the padding on that level should be empty (there's nothing to link to below it).
// If a Node is not the last one, the padding on that level should be the link edge so the sibling below is correctly connected.
func padding(level int, node Node) string {
	links := make([]string, level+1)

	for node.GetParent() != nil && level >= 0 {
		if isLast(node) {
			links[level] = strings.Repeat(" ", IndentSize+1)
		} else {
			links[level] = fmt.Sprintf("%s%s", EdgeTypeLink, strings.Repeat(" ", IndentSize))
		}
		level--
		node = node.GetParent()
	}

	return strings.Join(links, "")
}

// isLast checks if the Node is the last one in the slice of its parent children
func isLast(n Node) bool {
	parent := n.GetParent().(*Dir)
	if parent == nil {
		return false
	}
	return n == parent.FindLastNode()
}
