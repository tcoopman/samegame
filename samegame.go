package main

import (
	"fmt"
	"gopkg.in/qml.v1"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const (
	NB_OF_TYPES   = 4
	MAX_COL       = 10
	MAX_ROW       = 15
	MAX_INDEX     = MAX_COL * MAX_ROW
	SCORE_TIMEOUT = 3
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

type Game struct {
	MaxColumn  int
	MaxRow     int
	MaxIndex   int
	Board      []qml.Object
	Block      *Block
	fillFound  int
	floorBoard []int
	parent     qml.Object
	dialog     qml.Object
	Score      qml.Object
	started    bool
}

func (g *Game) index(col, row int) int {
	return col + (row * g.MaxColumn)
}

func (g *Game) StartNewGame(parent qml.Object, dialog qml.Object) {
	for _, b := range g.Board {
		if b != nil {
			b.Destroy()
		}
	}

	g.parent = parent
	g.dialog = dialog

	score := 0
	g.parent.Set("score", score)
	g.Score.Set("text", "Score: "+strconv.Itoa(score))

	w := parent.Int("width")
	h := parent.Int("height")
	blockSize := parent.Int("blockSize")
	g.MaxColumn = w / blockSize
	g.MaxRow = h / blockSize
	g.MaxIndex = g.MaxColumn * g.MaxRow

	g.Block.BlockSize = blockSize

	g.Board = make([]qml.Object, g.MaxIndex, g.MaxIndex)
	for col := 0; col < g.MaxColumn; col++ {
		for row := 0; row < g.MaxRow; row++ {
			g.Board[g.index(col, row)] = g.Block.createBlock(col, row, parent)
		}
	}
	g.started = true
}

func (g *Game) HandleClick(xPos, yPos int) {
	if !g.started {
		return
	}

	col := xPos / g.Block.BlockSize
	row := yPos / g.Block.BlockSize

	if col >= g.MaxColumn || col < 0 || row >= g.MaxRow || row < 0 {
		return
	}
	if g.Board[g.index(col, row)] == nil {
		return
	}
	g.floodFill(col, row, -1)
	if g.fillFound <= 0 {
		return
	}

	// Set the score
	score := g.parent.Int("score")
	score += (g.fillFound - 1) * (g.fillFound - 1)
	g.parent.Set("score", score)
	g.Score.Set("text", "Score: "+strconv.Itoa(score))

	g.shuffleDown()
	g.victoryCheck()
}

func (g *Game) floodFill(col, row, typ int) {
	if col >= g.MaxColumn || col < 0 || row >= g.MaxRow || row < 0 {
		return
	}
	if g.Board[g.index(col, row)] == nil {
		return
	}
	first := false
	if typ == -1 {
		first = true
		typ = g.Board[g.index(col, row)].Int("type")

		g.fillFound = 0
		g.floorBoard = make([]int, g.MaxIndex, g.MaxIndex)
	}

	if g.floorBoard[g.index(col, row)] == 1 || (!first && typ != g.Board[g.index(col, row)].Int("type")) {
		return
	}

	g.floorBoard[g.index(col, row)] = 1
	g.floodFill(col+1, row, typ)
	g.floodFill(col-1, row, typ)
	g.floodFill(col, row+1, typ)
	g.floodFill(col, row-1, typ)
	if first && g.fillFound == 0 {
		return //Can't remove single blocks
	}
	g.Board[g.index(col, row)].Set("dying", true)
	g.Board[g.index(col, row)] = nil
	g.fillFound += 1
}

func (g *Game) shuffleDown() {
	// Fall down
	for col := 0; col < g.MaxColumn; col++ {
		fallDist := 0
		for row := g.MaxRow - 1; row >= 0; row-- {
			if g.Board[g.index(col, row)] == nil {
				fallDist += 1
			} else {
				if fallDist > 0 {
					obj := g.Board[g.index(col, row)]
					y := obj.Int("targetY")
					y += fallDist * g.Block.BlockSize
					obj.Set("targetY", y)
					obj.Set("y", y)
					g.Board[g.index(col, row+fallDist)] = obj
					g.Board[g.index(col, row)] = nil
				}
			}
		}
	}

	// Fall to the left
	fallDist := 0
	for col := 0; col < g.MaxColumn; col++ {
		if g.Board[g.index(col, g.MaxRow-1)] == nil {
			fallDist += 1
		} else {
			if fallDist > 0 {
				for row := 0; row < g.MaxRow; row++ {
					obj := g.Board[g.index(col, row)]
					if obj == nil {
						continue
					}

					x := obj.Int("targetX")
					x -= fallDist * g.Block.BlockSize
					obj.Set("targetX", x)
					obj.Set("x", x)
					g.Board[g.index(col-fallDist, row)] = obj
					g.Board[g.index(col, row)] = nil
				}
			}
		}
	}
}

func (g *Game) victoryCheck() {
	deservesBonus := true
	for col := g.MaxColumn - 1; col >= 0; col-- {
		if g.Board[g.index(col, g.MaxRow-1)] != nil {
			deservesBonus = false
		}
	}
	score := g.parent.Int("score")
	if deservesBonus {
		score += 500
		g.parent.Set("score", score)
		g.Score.Set("text", "Score: "+strconv.Itoa(score))
	}

	if deservesBonus || !(g.floodMoveCheck(0, g.MaxRow-1, -1)) {
		g.dialog.Call("show", "Game over. Your score is "+strconv.Itoa(score))
		go func() {
			opened := time.Now()
			for time.Now().Sub(opened) < time.Second*SCORE_TIMEOUT {
			}
			g.dialog.Call("hide")
		}()
	}
}

func (g *Game) floodMoveCheck(col, row, typ int) bool {
	if col >= g.MaxColumn || col < 0 || row >= g.MaxRow || row < 0 {
		return false
	}
	if g.Board[g.index(col, row)] == nil {
		return false
	}
	myType := g.Board[g.index(col, row)].Int("type")
	if typ == myType {
		return true
	}
	return g.floodMoveCheck(col+1, row, myType) || g.floodMoveCheck(col, row-1, myType)
}

func (g *Game) DestroyBlock(block qml.Object, t int) {
	go func() {
		time.Sleep(time.Duration(t) * time.Millisecond)
		block.Destroy()
	}()
}

type Block struct {
	Component qml.Object
	BlockSize int
}

func (b *Block) createBlock(col, row int, parent qml.Object) qml.Object {
	dynamicBlock := b.Component.Create(nil)
	dynamicBlock.Set("parent", parent)

	dynamicBlock.Set("type", r.Intn(NB_OF_TYPES))
	dynamicBlock.Set("x", col*b.BlockSize)
	dynamicBlock.Set("targetX", col*b.BlockSize)
	dynamicBlock.Set("y", row*b.BlockSize)
	dynamicBlock.Set("targetY", row*b.BlockSize)
	dynamicBlock.Set("width", b.BlockSize)
	dynamicBlock.Set("height", b.BlockSize)
	dynamicBlock.Set("spawned", true)

	return dynamicBlock
}

func main() {
	if err := qml.Run(run); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	engine := qml.NewEngine()

	component, err := engine.LoadFile("samegame.qml")
	if err != nil {
		return err
	}

	game := Game{
		MaxColumn: MAX_COL,
		MaxRow:    MAX_ROW,
		MaxIndex:  MAX_COL * MAX_ROW,
	}

	context := engine.Context()
	context.SetVar("game", &game)

	win := component.CreateWindow(nil)

	blockComponent, err := engine.LoadFile("Block.qml")
	if err != nil {
		return err
	}

	block := &Block{Component: blockComponent}
	game.Block = block

	game.Score = win.Root().ObjectByName("score")

	win.Show()
	win.Wait()

	return nil
}
