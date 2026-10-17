package deploy

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	diffContext  = 3
	diffMaxCells = 4 << 20 // an O(n*m) table beyond this is not worth building for a config file
)

type diffOp struct {
	kind byte // ' ', '-', '+'
	text string
}

// unifiedDiff renders the lines that turn a into b. It returns a one-line note for binary or huge input.
func unifiedDiff(aName, bName string, a, b []byte) string {
	if bytes.IndexByte(a, 0) >= 0 || bytes.IndexByte(b, 0) >= 0 {
		return fmt.Sprintf("Binary files %s and %s differ\n", aName, bName)
	}
	al, bl := splitLines(a), splitLines(b)
	if len(al)*len(bl) > diffMaxCells {
		return fmt.Sprintf("Files %s and %s differ (too large to show)\n", aName, bName)
	}
	ops := diffOps(al, bl)
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", aName, bName)
	writeHunks(&sb, ops)
	return sb.String()
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func diffOps(a, b []string) []diffOp {
	// lcs[i][j] is the LCS length of a[i:] and b[j:]
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

func writeHunks(sb *strings.Builder, ops []diffOp) {
	// near[i] is true if ops[i] is a change or within diffContext lines of one
	near := make([]bool, len(ops))
	for i, op := range ops {
		if op.kind == ' ' {
			continue
		}
		for k := i - diffContext; k <= i+diffContext; k++ {
			if k >= 0 && k < len(ops) {
				near[k] = true
			}
		}
	}
	aLine, bLine := 1, 1
	for i := 0; i < len(ops); {
		if !near[i] {
			aLine, bLine = advance(ops[i], aLine, bLine)
			i++
			continue
		}
		end := i
		for end < len(ops) && near[end] {
			end++
		}
		aStart, bStart, aCount, bCount := aLine, bLine, 0, 0
		var body strings.Builder
		for _, op := range ops[i:end] {
			body.WriteString(string(op.kind) + op.text + "\n")
			if op.kind != '+' {
				aCount++
			}
			if op.kind != '-' {
				bCount++
			}
			aLine, bLine = advance(op, aLine, bLine)
		}
		fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n%s", aStart, aCount, bStart, bCount, body.String())
		i = end
	}
}

func advance(op diffOp, aLine, bLine int) (int, int) {
	if op.kind != '+' {
		aLine++
	}
	if op.kind != '-' {
		bLine++
	}
	return aLine, bLine
}
