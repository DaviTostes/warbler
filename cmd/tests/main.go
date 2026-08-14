package main

import (
	"fmt"
	"strings"
	"warbler/internal/gen"
)

func main() {
	g, err := gen.InitGenkit()
	if err != nil {
		panic(err)
	}

	stream := gen.GenerateStream(g, "", "hey!", nil, nil)
	for result, err := range stream {
		if err != nil {
			panic(err)
		}

		if result.Done {
			// fmt.Print(result.Response.Text())
			break
		}

		if !strings.Contains(result.Chunk.Reasoning(), result.Chunk.Text()) {
			fmt.Print(result.Chunk.Text())
		}
	}
}
