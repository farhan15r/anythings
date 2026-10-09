package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	var n int

	_, err := fmt.Fscan(reader, &n)
	if err != nil {
		panic(err)
	}

	himpunanBambu := make([]int, n)
	targetHimpunanBambu := make([]int, n)

	for i := 0; i < n; i++ {
		fmt.Fscan(reader, &himpunanBambu[i])
	}

	for i := 0; i < n; i++ {
		fmt.Fscan(reader, &targetHimpunanBambu[i])
	}

	sort.Sort(sort.Reverse(sort.IntSlice(himpunanBambu)))
	sort.Sort(sort.Reverse(sort.IntSlice(targetHimpunanBambu)))

	jumlahHari := targetHimpunanBambu[0] - himpunanBambu[0]
	bisaDiCapai := true

	// fmt.Println(n)
	// fmt.Println(himpunanBambu)
	// fmt.Println(targetHimpunanBambu)

	// fmt.Println(jumlahHari)

	// validating
	for i := 0; i < n; i++ {
		if (0 > (targetHimpunanBambu[i] - himpunanBambu[i])) ||
			(targetHimpunanBambu[i]-himpunanBambu[i]) > jumlahHari {
			bisaDiCapai = false
			break
		}
	}

	resultText := ""
	if bisaDiCapai {
		resultText = "YA"
	} else {
		resultText = "TIDAK"
	}
	fmt.Println(resultText)
}
