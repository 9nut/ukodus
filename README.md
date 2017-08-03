# ukodus
Sudoku solver in Go

I wrote this because I was spending too much time on Sudoku puzzles.
I reasoned that if I wrote a solver, I would stop being compelled to
solved them by hand, since the solution requires the application of the
same finite set of strategies.

## build

`go build`

## usage

`./ukodus [-d] [-9] puzzle [puzzle...]`

Use `-d` to see a trace of the program as it solves puzzles.
Some Sudoku games, like the one on Plan 9, use a different storage
format for the puzzle. To use that format, include the `-9` flag.

## example

`./ukodus boards/s16.txt`

