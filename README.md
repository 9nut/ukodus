# ukodus
Sudoku solver in Go

I wrote this because I was spending too much time on Sudoku puzzles.
I reasoned that if I wrote a solver, I would not be compelled to
solve them by hand.  The solution requires the application of the
same finite set of strategies.

## build

`go build`

## usage

`./ukodus [-d] [-9] [-p] puzzle [puzzle...]`

Use `-d` to see a trace of the program as it solves puzzles. Use `-p`
to enable `pprof` profiling. Some Sudoku games, like the one on Plan
9, use a different storage format for the puzzle. To use that format,
include the `-9` flag.

## example

`./ukodus boards/s16.txt`

