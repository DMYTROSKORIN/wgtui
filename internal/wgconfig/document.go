package wgconfig

import (
	"bufio"
	"regexp"
	"strings"
)

// Document is a byte-preserving representation of a wg-quick config file:
// the file is split into the [Interface] block and each [Peer] block, and
// every byte between and within blocks is kept exactly as read. Mutating
// operations (AppendPeerBlock, RemoveBlockAt) only ever touch the block(s)
// they target - everything else, including comments, blank lines,
// unrecognized directives, and PreUp/PostUp/PostDown hooks, round-trips
// unchanged (SPEC.md §15.5). This is the writer half of the parser; the
// read-only, lossy ConfigSummary in summary.go exists separately for
// discovery's cheaper peek.
type Document struct {
	// Preamble is any text before the first [Interface]/[Peer] header -
	// normally empty, but a hand-edited file could have a leading comment.
	Preamble string
	Blocks   []*Block
}

// Block is one [Interface] or [Peer] section: the exact header line as it
// appeared (which may be prefixed with a comment line naming the peer, if
// this block was built by AppendPeerBlock) plus every line up to the next
// header or end of file.
type Block struct {
	HeaderRaw string // e.g. "[Peer]\n" or "# Alex's laptop\n[Peer]\n"
	Kind      string // "interface" or "peer", lowercase
	Body      string
}

var headerLinePattern = regexp.MustCompile(`(?i)^[ \t]*\[[ \t]*(interface|peer)[ \t]*\][ \t]*\r?\n?$`)

// Parse splits raw wg-quick config text into a Document. It never fails:
// text that doesn't look like a WireGuard config just becomes a Document
// with no blocks and everything in Preamble, which callers can reject
// separately (e.g. by checking len(Blocks) == 0) rather than Parse trying
// to guess what's "invalid".
func Parse(raw string) *Document {
	doc := &Document{}
	var cur *Block
	for _, line := range splitAfterLines(raw) {
		if m := headerLinePattern.FindStringSubmatch(line); m != nil {
			cur = &Block{HeaderRaw: line, Kind: strings.ToLower(m[1])}
			doc.Blocks = append(doc.Blocks, cur)
			continue
		}
		if cur == nil {
			doc.Preamble += line
		} else {
			cur.Body += line
		}
	}
	return doc
}

// splitAfterLines splits s into pieces each ending with its own "\n"
// (CRLF is preserved as part of the preceding piece, since "\r" isn't the
// split delimiter). Concatenating the result always reproduces s exactly,
// which is what makes Document round-trip byte-for-byte.
func splitAfterLines(s string) []string {
	return strings.SplitAfter(s, "\n")
}

// String reconstructs the full config text. For an unmodified Document
// (Parse then String with no calls in between), this is byte-identical to
// the original input.
func (d *Document) String() string {
	var b strings.Builder
	b.WriteString(d.Preamble)
	for _, blk := range d.Blocks {
		b.WriteString(blk.HeaderRaw)
		b.WriteString(blk.Body)
	}
	return b.String()
}

// InterfaceBlock returns the first "[Interface]" block, if any.
func (d *Document) InterfaceBlock() (*Block, bool) {
	for _, blk := range d.Blocks {
		if blk.Kind == "interface" {
			return blk, true
		}
	}
	return nil, false
}

// FindPeerByPublicKey returns the index of the [Peer] block whose
// PublicKey directive equals pubKey, or -1 if none matches.
func (d *Document) FindPeerByPublicKey(pubKey string) int {
	for i, blk := range d.Blocks {
		if blk.Kind == "peer" && blk.PublicKey() == pubKey {
			return i
		}
	}
	return -1
}

// RemoveBlockAt deletes the block at index i and returns it, so callers
// (e.g. "disable") can keep it for a later "resume" without needing to
// remember to re-derive it from scratch.
func (d *Document) RemoveBlockAt(i int) *Block {
	removed := d.Blocks[i]
	d.Blocks = append(d.Blocks[:i], d.Blocks[i+1:]...)
	return removed
}

// AppendPeerBlock adds blk as a new final block. Callers build blk with
// BuildPeerBlock.
func (d *Document) AppendPeerBlock(blk *Block) {
	d.Blocks = append(d.Blocks, blk)
}

// PublicKey extracts the PublicKey directive's value from a peer block's
// body ("" if absent - callers decide whether that's an error).
func (b *Block) PublicKey() string {
	return b.directiveValue("PublicKey")
}

// SaveConfigEnabled reports whether an [Interface] block sets
// SaveConfig = true (SPEC.md §15.4).
func (b *Block) SaveConfigEnabled() bool {
	return strings.EqualFold(b.directiveValue("SaveConfig"), "true")
}

// Value returns the value of an arbitrary directive in this block's body
// ("" if absent), for callers that need a field PublicKey/SaveConfigEnabled
// don't already cover (e.g. reading AllowedIPs/Endpoint back out of a
// previously generated block or profile).
func (b *Block) Value(key string) string {
	return b.directiveValue(key)
}

// FirstPeerBlock returns the first "[Peer]" block in d, if any - useful
// for re-reading a single-peer client profile (which also parses fine as
// a Document: it has one [Interface] and one [Peer] block).
func (d *Document) FirstPeerBlock() (*Block, bool) {
	for _, blk := range d.Blocks {
		if blk.Kind == "peer" {
			return blk, true
		}
	}
	return nil, false
}

func (b *Block) directiveValue(key string) string {
	sc := bufio.NewScanner(strings.NewReader(b.Body))
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimRight(sc.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
