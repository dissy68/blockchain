package blockchain

import (
	"testing"
)

// --- Helpers -----------------------------------------------------------------

// getGenesisNode is a small helper that finds the unique genesis node
// in a BlockTree for assertions in tests.
func getGenesisNode(t *testing.T, bt *BlockTree) *BlockNode {
	t.Helper()
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	var genesis *BlockNode
	for _, n := range bt.nodes {
		if n.IsGenesis() {
			if genesis != nil {
				t.Fatalf("more than one genesis node found")
			}
			genesis = n
		}
	}
	if genesis == nil {
		t.Fatalf("no genesis node found")
	}
	return genesis
}

// --- Block tests -------------------------------------------------------------

func TestBlock_IsGenesis(t *testing.T) {
	genesis := &Block{
		PreviousHash: "",
		Hash:         "seed",
		Slot:         0,
	}

	nonGenesis := &Block{
		PreviousHash: "prev",
		Hash:         "h1",
		Slot:         1,
	}

	if !genesis.IsGenesis() {
		t.Errorf("expected genesis block with empty PreviousHash to be IsGenesis == true")
	}
	if nonGenesis.IsGenesis() {
		t.Errorf("expected non-genesis block with non-empty PreviousHash to be IsGenesis == false")
	}
}

// --- BlockNode tests ---------------------------------------------------------

func TestBlockNode_IsGenesisAndIsLeaf(t *testing.T) {
	genBlock := &Block{Hash: "gen"}
	genNode := &BlockNode{
		Block:    genBlock,
		Parent:   nil,
		Children: []*BlockNode{},
	}

	childBlock := &Block{Hash: "child"}
	childNode := &BlockNode{
		Block:    childBlock,
		Parent:   genNode,
		Children: []*BlockNode{},
	}
	genNode.Children = append(genNode.Children, childNode)

	// Genesis node: parent == nil
	if !genNode.IsGenesis() {
		t.Errorf("expected node with nil Parent to be IsGenesis == true")
	}
	if genNode.IsLeaf() {
		t.Errorf("expected genesis with one child not to be leaf, got IsLeaf == true")
	}

	// Child node: parent != nil, no children
	if childNode.IsGenesis() {
		t.Errorf("expected child node with non-nil Parent to be IsGenesis == false")
	}
	if !childNode.IsLeaf() {
		t.Errorf("expected node with zero children to be IsLeaf == true")
	}
}

// --- BlockTree construction / AddBlock --------------------------------------

func TestNewBlockTree_CreatesGenesisNode(t *testing.T) {
	genesisBlock := &Block{
		PreviousHash: "",
		Hash:         "seed",
		Slot:         0,
	}
	bt := NewBlockTree(genesisBlock)

	if bt.nodes == nil {
		t.Fatalf("expected BlockTree.nodes to be initialized")
	}
	if len(bt.nodes) != 1 {
		t.Fatalf("expected exactly 1 node in tree, got %d", len(bt.nodes))
	}

	node, ok := bt.nodes["seed"]
	if !ok {
		t.Fatalf("expected genesis hash to be key in nodes map")
	}
	if node.Block != genesisBlock {
		t.Errorf("expected node.Block to be the same pointer as genesisBlock")
	}
	if !node.IsGenesis() {
		t.Errorf("expected genesis node to be IsGenesis == true")
	}
}

func TestBlockTree_AddBlock_Success(t *testing.T) {
	genesisBlock := &Block{
		PreviousHash: "",
		Hash:         "seed",
		Slot:         0,
	}
	bt := NewBlockTree(genesisBlock)
	genesisNode := getGenesisNode(t, bt)

	childBlock := &Block{
		PreviousHash: genesisBlock.Hash,
		Hash:         "child1",
		Slot:         1,
	}

	ok := bt.AddBlock(childBlock)
	if !ok {
		t.Fatalf("expected AddBlock to return true for existing parent")
	}

	bt.mu.RLock()
	defer bt.mu.RUnlock()

	childNode, exists := bt.nodes[childBlock.Hash]
	if !exists {
		t.Fatalf("expected child block to be stored in tree nodes")
	}
	if childNode.Parent != genesisNode {
		t.Errorf("expected child node parent to be genesisNode")
	}
	if len(genesisNode.Children) != 1 || genesisNode.Children[0] != childNode {
		t.Errorf("expected genesisNode.Children to contain childNode")
	}
}

func TestBlockTree_AddBlock_FailsWhenParentMissing(t *testing.T) {
	genesisBlock := &Block{
		PreviousHash: "",
		Hash:         "seed",
		Slot:         0,
	}
	bt := NewBlockTree(genesisBlock)

	block := &Block{
		PreviousHash: "nonexistent",
		Hash:         "orphan",
		Slot:         1,
	}

	ok := bt.AddBlock(block)
	if ok {
		t.Fatalf("expected AddBlock to return false for missing parent")
	}

	bt.mu.RLock()
	defer bt.mu.RUnlock()

	if _, exists := bt.nodes[block.Hash]; exists {
		t.Errorf("expected orphan block NOT to be added to nodes map")
	}
}

// --- FindHead tests ---------------------------------------------------------

func TestBlockTree_FindHead_SimpleChain(t *testing.T) {
	genesis := &Block{PreviousHash: "", Hash: "g", Slot: 0}
	bt := NewBlockTree(genesis)

	b1 := &Block{PreviousHash: "g", Hash: "b1", Slot: 1}
	b2 := &Block{PreviousHash: "b1", Hash: "b2", Slot: 2}

	if !bt.AddBlock(b1) {
		t.Fatalf("failed to add b1")
	}
	if !bt.AddBlock(b2) {
		t.Fatalf("failed to add b2")
	}

	head := bt.GetHead()
	if head == nil {
		t.Fatalf("expected head to be non-nil")
	}
	if head.Block.Hash != "b2" {
		t.Errorf("expected head to be b2, got %s", head.Block.Hash)
	}
}

// Fork: longest chain should be chosen as head.
func TestBlockTree_FindHead_LongestChain(t *testing.T) {
	genesis := &Block{PreviousHash: "", Hash: "g", Slot: 0}
	bt := NewBlockTree(genesis)

	// Two children of genesis
	a1 := &Block{PreviousHash: "g", Hash: "a1", Slot: 1}
	b1 := &Block{PreviousHash: "g", Hash: "b1", Slot: 1}
	if !bt.AddBlock(a1) || !bt.AddBlock(b1) {
		t.Fatalf("failed to add a1 or b1")
	}

	// Extend branch A: g -> a1 -> a2 -> a3  (depth 3)
	a2 := &Block{PreviousHash: "a1", Hash: "a2", Slot: 2}
	a3 := &Block{PreviousHash: "a2", Hash: "a3", Slot: 3}
	if !bt.AddBlock(a2) || !bt.AddBlock(a3) {
		t.Fatalf("failed to add a2 or a3")
	}

	// Shorter branch B: g -> b1 -> b2 (depth 2)
	b2 := &Block{PreviousHash: "b1", Hash: "b2", Slot: 2}
	if !bt.AddBlock(b2) {
		t.Fatalf("failed to add b2")
	}

	head := bt.GetHead()
	if head == nil {
		t.Fatalf("expected head to be non-nil")
	}
	if head.Block.Hash != "a3" {
		t.Errorf("expected head to be the tip of the longest chain (a3), got %s", head.Block.Hash)
	}
}

// --- CreateGenesisBlock tests -----------------------------------------------

// NOTE: You might need to adjust the type
