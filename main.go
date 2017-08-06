// Copyright © 2017 Skip Tavakkolian
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/profile"
)

// cell type uses the lowest 5 bits for value of the cell, 0 meaning
// unknown and the next lowest 9 bits for possible values
type cell int
type tuple []*cell
type board [9][9]*cell

func (c *cell) value() int {
	return int(*c) & 0x1f
}

func (c *cell) possible() int {
	return (int(*c) >> 5) & 0x1ff
}

func (c *cell) pcount() int {
	return bitcount((int(*c) >> 5) & 0x1ff)
}

func (c *cell) slotisset(p uint) bool {
	bit := (1 << (p - 1)) << 5
	return (int(*c) & bit) != 0
}

// Set the value and its corresponding "possible" bit
// 0 is used as "not set yet", and all possible bits are
// turned on
func (c *cell) setvalue(v int) {
	if v == 0 {
		*c = cell(0x1ff << 5)
	} else if v >= 1 && v <= 9 {
		bit := 1 << uint(5+v-1)
		*c = cell(bit | (v & 0x1f))
	}
}

func (c *cell) setslot(p uint) {
	*c |= cell(1 << (5 + p - 1))
}

func (c *cell) clearslot(p uint) {
	bitoff := ^(1 << (5 + p - 1))
	*c &= cell(bitoff | 0x1f)
}

func (c *cell) assign(rhs *cell) {
	*c = *rhs
}

func parseboard(description string, p9format bool) (board, error) {
	if p9format {
		return parseColumnRow(description)
	}
	return parseRowColumn(description)
}

func parseRowColumn(description string) (puzzle board, err error) {
	lines := strings.Split(description, "\n")
	if len(lines) < 9 {
		err = errors.New("not enough rows")
		return
	}

	for r, v := range lines[:9] {
		log.Println("looking at: ", v)
		cols := strings.Split(v, " ")
		if len(cols) < 9 {
			err = errors.New(fmt.Sprintf("not enough columns in line %d", r))
			return
		}
		// TODO: more input checking: check for duplicate values
		// for each row, column and box
		for c, x := range cols[:9] {
			switch x[0] {
			case '.', '_', '0':
				nc := new(cell)
				nc.setvalue(0)
				puzzle[r][c] = nc

			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				nc := new(cell)
				nc.setvalue(int(x[0]) - int('0'))
				puzzle[r][c] = nc
			default:
				err = errors.New(fmt.Sprintf("bad puzzle value row: %d col: %d", r, c))
				return
			}
		}
	}
	return
}

func parseColumnRow(description string) (puzzle board, err error) {
	lines := strings.Split(description, "\n")
	if len(lines) < 9 {
		err = errors.New("not enough rows")
		return
	}

	for c, v := range lines[:9] {
		log.Println("looking at: ", v)
		rows := strings.Split(v, "")
		if len(rows) < 9 {
			err = errors.New(fmt.Sprintf("not enough rows in line %d", c))
			return
		}
		// TODO: more input checking: check for duplicate values
		// for each row, column and box
		for r, x := range rows[:9] {
			switch x[0] {
			case '.', '_':
				nc := new(cell)
				nc.setvalue(0)
				puzzle[r][c] = nc
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				nc := new(cell)
				nc.setvalue(int(x[0]) - int('0'))
				puzzle[r][c] = nc
			default:
				err = errors.New(fmt.Sprintf("bad puzzle value row: %d col: %d", r, c))
				return
			}
		}
	}

	return
}

func replicate(p1 board) (p2 board) {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			nc := new(cell)
			nc.assign(p1[r][c])
			p2[r][c] = nc
		}
	}
	return
}

func main() {
	debug := flag.Bool("d", false, "enable logging trace")
	plan9 := flag.Bool("9", false, "Plan 9 Sudoku puzzle format")
	pprof := flag.Bool("p", false, "enable pprof")

	flag.Parse()

	log.SetOutput(ioutil.Discard)
	if *debug {
		log.SetOutput(os.Stderr)
	}
	if *pprof {
		defer profile.Start().Stop()
	}

	files := flag.Args()
	for _, file := range files {
		sudoku, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}

		puzzle, err := parseboard(string(sudoku), *plan9)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Puzzle: ", file)

		solution, impossible := solve(puzzle)
		if impossible {
			fmt.Printf("%s: solution isn't possible\n", file)
		} else {
			fmt.Printf("%s: is solved? %t\n", file, unknownCount(solution) == 0)
		}
	}
}

// strategy
// step 1: eliminiation
// clear a flag that notes if a change has happened.
// descending order ofnumber of hints given; the more hints, the
// earlier in the list].
//
// 1. traverse all the unassigned cells and create a list of possible
// values for based on the row, column and box tuples; e.g. cell(2,2)
// has a row tuple board[2][:], a column tuple board[:][2] and a
// box tuple board[1:3][1:3].
//
// - if there is only one possible value, assign the value to cell,
// note that a change has happened and start over.
//
// - if there are exactly two values, search its column, row and
// box tuples for the same exact two values;
// - if there is a match eliminate those values from the coresponding
// col, row or box cells, note the change and start over.
//
// if a change has not occured in this loop, can't solve the puzzle.
// with this strategy, and try brute force (step 2)
//
// step 2: brute force substitution
// for each cell that has only two values, push a copy of the puzzle
// on stack and, assign one of the two values and try to solve it using
// elimination (step 1).  that doesn't succeed, pop the stack, try the second
// value by assinging it, pushing that copy of the puzzle on the stack and
// trying to solve it. substituted values that aren't correct will result
// in impossible values for cells and will be abandoned.
func solve(puzzle board) (board, bool) {
	printPuzzle(puzzle)
	unknowns := unknownCount(puzzle)
	// step 1: elimination
	puzzle, _, impossible := elimination(puzzle)
	if impossible {
		return puzzle, impossible
	}

	fmt.Println("After step1: ", unknowns-unknownCount(puzzle), "/", unknowns)
	printPuzzle(puzzle)

	// step2: substitution
	// try all 2,3,4...-possibility cells, retracting when it doesn't work.
	// use recursion to try the branches; could use a stack implementation
	// but recursion is easier to sort out.  elimination and substitution
	// are used
	if unknownCount(puzzle) != 0 {
		puzzle, impossible = substitution(puzzle, 2)
	}

	fmt.Println("After step2: ", unknowns-unknownCount(puzzle), "/", unknowns)
	fmt.Println("Solution:")
	printPuzzle(puzzle)

	return puzzle, impossible
}

// eliminate all hints to discover cell values
// and use those as further hints.
func elimination(puzzle board) (pz board, found, impossible bool) {
	pz = replicate(puzzle)

	changed := true
	for changed {
		changed, impossible = findOne(pz)
		if impossible {
			log.Println("solution not possible")
			return
		}
		if changed {
			found = true
		}
	}

	return
}

func printPuzzle(puzzle board) {
	for i := 0; i < 9; i++ {
		fmt.Print("[")
		for j := 0; j < 9; j++ {
			v := puzzle[i][j]
			s := strconv.FormatInt(int64(v.possible()), 2)
			fmt.Printf("%d(%09s) ", v.value(), s)
		}
		fmt.Println("]")
	}
}

// find possible values for elem by looking at existing
// values in tuple and eliminating them from possibles for
// this element.
func findpossibles(set tuple, elem int) int {
	for i, v := range set {
		if i != elem && v.value() != 0 {
			set[elem].clearslot(uint(v.value()))
		}
	}
	// openslots := set[elem].possible()
	return set[elem].pcount()
}

// check to see if cells in tuple other than elem have
// the ability to accept value (if value is possible)
func hasval(set tuple, elem int, value int) bool {
	for i, v := range set {
		if i != elem {
			if v.value() == value || v.slotisset(uint(value)) {
				return true
			}
		}
	}
	return false
}

// find any slots that are unique to elem's cell; if
// checkzero is set, eliminate possible values of other
// unknown cells in tuple.
func uniqueslot(set tuple, elem int, checkzeros bool) int {
	pos := set[elem].possible()
	val := pos
	for i, v := range set {
		if i != elem && (checkzeros || v.value() != 0) {
			val ^= v.possible()
			val &= pos
		}
		if val == 0 {
			return 0
		}
	}
	return pos
}

func getrowtuple(puzzle board, i int) tuple {
	t := puzzle[i][:]
	// printTuple("getrowtuple", t)
	return t
}

func getcoltuple(puzzle board, j int) tuple {
	t := make([]*cell, 9)
	for i := 0; i < 9; i++ {
		t[i] = puzzle[i][j]
	}
	// printTuple("getcoltuple", t)
	return t
}

// make a tuple from the 3x3 box that cell(i,j) is in; put cell(i,j) in the
// first slot of the tuple
func getboxtuple(puzzle board, i, j int) tuple {
	t := make([]*cell, 9)
	x := i / 3
	y := j / 3
	for k := 0; k < 3; k++ {
		for l := 0; l < 3; l++ {
			xn := x*3 + k
			yn := y*3 + l
			t[k*3+l] = puzzle[xn][yn]
			// if we're at cell[i][j]
			if xn == i && yn == j && (k*3+l) != 0 {
				// swap t[0] with this cell
				t[0], t[k*3+l] = t[k*3+l], t[0]
			}
		}
	}
	// printTuple("getboxtuple", t)
	return t
}

// find matching cell that has the same exact two possible
// bits turned on. the cell at elem must have exactly 2
// possible values
func findmatching(set tuple, elem int) (bool, int) {
	openslots := set[elem].possible()

	// should be an error, if it ever happens.
	if bitcount(openslots) != 2 {
		return false, -1
	}

	for i, v := range set {
		poss := v.possible()
		if i != elem && v.value() == 0 && poss == openslots {
			return true, i
		}
	}
	return false, -1
}

// removeslots removes the slots in elements ea and eb from
// other slots in the tuple
func removeslots(set tuple, ea, eb int) bool {
	openslots := set[ea].possible()
	// assert openslots == set[eb].possible() && bitcount(openslots) == 2
	spokenfor := bitvalues(openslots)

	found := false
	count := 0
	for i, v := range set {
		// unset values other than ea, eb
		if i != ea && i != eb && v.value() == 0 {
			for _, s := range spokenfor {
				if v.slotisset(uint(s)) {
					v.clearslot(uint(s))
				}
			}
			if v.pcount() == 1 {
				v.setvalue(bitvalue(v.possible()))
				found = true
				count++
			}
		}
	}

	// log.Println("removeslots: ", found, count)
	return found
}

// for each cell, eliminate values that are already its row, col and box.
// if there is only one possible value, assign it and return true. if there
// are zero possibles, then return impossible.
func checkCell(puzzle board, i, j int) (changed, impossible bool) {
	openslots := puzzle[i][j].possible()
	// s := strconv.FormatInt(int64(openslots), 2)
	// log.Printf("row/col/box check cell(%d, %d), value %d, possibles %s\n", i, j, puzzle[i][j].value(), s)

	possibles := bitcount(openslots)
	if possibles < 2 {
		return
	}

	tr := getrowtuple(puzzle, i)
	tc := getcoltuple(puzzle, j)
	tb := getboxtuple(puzzle, i, j)

	possibles = findpossibles(tb, 0)
	if possibles > 1 {
		possibles = findpossibles(tc, i)
	}
	if possibles > 1 {
		possibles = findpossibles(tr, j)
	}

	openslots = puzzle[i][j].possible()
	// s = strconv.FormatInt(int64(openslots), 2)
	// log.Printf("after row,col,box check cell(%d,%d) possibles: %s\n", i, j, s)

	switch possibles {
	case 0: // impossible
		// log.Println("case 0: solution impossible")
		impossible = true
		return

	case 1: // single value, assign it, turn off all possibles
		val := bitvalue(openslots)
		puzzle[i][j].setvalue(val)
		// log.Printf("case 1: changed cell(%d,%d) to %d\n", i, j, puzzle[i][j].value())
		changed = true
		return

	case 2: // exactly two possible values
		// log.Println("case 2: search")
		ok, jj := findmatching(tr, j)
		if ok {
			// remove the 2 matching values in j and jj from other slots
			if removeslots(tr, j, jj) {
				changed = true
				return
			}
		}
		ok, ii := findmatching(tc, i)
		if ok {
			// remove the two matching values in i, ii from other slots
			if removeslots(tc, i, ii) {
				changed = true
				return
			}
		}
		ok, bb := findmatching(tb, 0)
		if ok {
			// remove the two matching values in 0 and bb from other slots
			if removeslots(tb, 0, bb) {
				changed = true
				return
			}
		}

	default:
		// for each possible number, check row, col, box tuples
		// to see if the other cells can also have that value
		// if none can have that value, then this cell must be
		// that value:

		pb := puzzle[i][j].possible()
		for n := 1; pb != 0; n++ {
			if pb&1 == 1 && !hasval(tr, j, n) && !hasval(tc, i, n) && !hasval(tb, 0, n) {
				puzzle[i][j].setvalue(n)
				// log.Printf("default: #1: changed cell(%d,%d) to %d\n", i, j, puzzle[i][j].value())
				changed = true
				return
			}
			pb >>= 1
		}

		for _, checkzeros := range []bool{false, true} {
			ur := uniqueslot(tr, j, checkzeros)
			uc := uniqueslot(tc, i, checkzeros)
			ub := uniqueslot(tb, 0, checkzeros)
			bit := ur & uc & ub
			if bitcount(bit) == 1 {
				puzzle[i][j].setvalue(bitvalue(bit))
				// log.Printf("default: #2: changed cell(%d,%d) to %d\n", i, j, puzzle[i][j].value())
				changed = true
				return
			}
		}
	}

	return
}

// for each cell, find one that doesn't have a value and look through
// all row, column and box cells; elimintate all hints and previously
// filled values. if at least one cell value changed, return
func findOne(puzzle board) (changed, impossible bool) {
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			changed, impossible = checkCell(puzzle, i, j)
			if changed || impossible {
				return
			}
		}
	}

	// no cells with a single possible value
	// look through cells in each row, each column and each box
	// looking for possibles that are only two
	return false, false
}

// substitution: for every cell that has npossibles, try to
// solve the puzzle by trying each of the possible values and
// restart elimination and if needed more substitution. try
// higher values of npossibles up to the maximum 9, if the
// puzzle is unsolved.
func substitution(puzzle board, npossibles int) (pz board, impossible bool) {
	log.Printf("substitution of %d possibles", npossibles)
	if npossibles > 9 {
		return pz, true
	}

	pz = replicate(puzzle)
	changed := false

	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			np := bitcount(pz[i][j].possible())
			if np == npossibles {
				possibles := bitvalues(pz[i][j].possible())

				for _, p := range possibles {
					log.Printf("trying %d for cell(%d, %d)\n", p, i, j)
					pz[i][j].setvalue(p)
					pz, changed, impossible = elimination(pz)
					if impossible {
						pz = replicate(puzzle)
						continue
					}
					if changed {
						pz, impossible = substitution(pz, npossibles)
						if impossible {
							pz = replicate(puzzle)
						}
					}
				}
			}
		}
	}

	if unknownCount(pz) != 0 {
		npossibles++
		pz, impossible = substitution(pz, npossibles)
	}

	return pz, impossible
}

func unknownCount(puzzle board) (unknowns int) {
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			if puzzle[i][j].value() == 0 {
				unknowns++
			}
		}
	}
	return
}

func bitcount(bv int) (count int) {
	/*
		for bv != 0 {
			if bv&0x1 != 0 {
				count++
			}
			bv >>= 1
		}
	*/
	// because pprof
	// Figure 5-2, Hacker's Delight -- Warren
	bv = bv - ((bv >> 1) & 0x55555555)
	bv = (bv & 0x33333333) + ((bv >> 2) & 0x33333333)
	bv = (bv + (bv >> 4)) & 0x0F0F0F0F
	bv = bv + (bv >> 8)
	bv = bv + (bv >> 16)
	count = bv & 0x3F
	return
}

// it's expected that only one bit position is set
func bitvalue(bv int) (value int) {
	for bv != 0 {
		value++
		bv >>= 1
	}
	return value
}

// return the list of possible values based on bit set
func bitvalues(bv int) []int {
	list := []int{}
	value := 0
	for bv != 0 {
		value++
		if bv&1 == 1 {
			list = append(list, value)
		}
		bv >>= 1
	}
	return list
}

func printTuple(n string, t tuple) {
	fmt.Print(n, "[ ")
	for _, v := range t {
		fmt.Print(v.value())
		s := strconv.FormatInt(int64(v.possible()), 2)
		fmt.Printf("(%09s) ", s)
	}
	fmt.Println("]")
}
