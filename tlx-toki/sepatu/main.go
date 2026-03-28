package main

import (
	"fmt"
	"slices"
)

func main() {
	var n, m int

	_, err := fmt.Scanln(&n, &m)
	if err != nil {
		panic(err)
	}

	var ukuranKakiBebek []int
	for i := 0; i < n; i++ {
		var tmp int
		fmt.Scanln(&tmp)

		ukuranKakiBebek = append(ukuranKakiBebek, tmp)
	}
	slices.Sort(ukuranKakiBebek)

	var ukuranSepatuBaru []int
	for i := 0; i < m; i++ {
		var tmp int
		fmt.Scanln(&tmp)

		ukuranSepatuBaru = append(ukuranSepatuBaru, tmp)
	}
	slices.Sort(ukuranSepatuBaru)

	bebekDapetSepatu := 0

	i := 0
	j := 0

	for (i < len(ukuranKakiBebek)) && (j < len(ukuranSepatuBaru)) {
		if (ukuranKakiBebek[i] == ukuranSepatuBaru[j]) ||
			(ukuranKakiBebek[i] == (ukuranSepatuBaru[j] - 1)) {
			bebekDapetSepatu++
			i++
			j++
		} else if ukuranSepatuBaru[j] < ukuranKakiBebek[i] {
			j++
		} else {
			i++
		}

	}

	fmt.Println(bebekDapetSepatu)
}
