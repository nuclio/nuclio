package main

import (
	"github.com/brianvoe/gofakeit/v6"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/dynamic-list"
	"math/rand"
	"sync"
	"time"
)

func main() {

	startTime := time.Now()
	itemList := make(dynamicList.List, 0)
	numberOfItems := 1_500_0

	for i := 0; i < numberOfItems; i++ {
		rawValue := string(rune('A' + rand.Intn(26)))
		attrs := map[dynamicList.Priority]interface{}{
			dynamicList.PriorityDeadline: gofakeit.FutureDate(),
			dynamicList.PriorityPriority: gofakeit.Number(1, 1000),
			dynamicList.PriorityName:     gofakeit.Name(),
		}

		itemList.Push(&dynamicList.Item{RawValue: rawValue, Attrs: attrs})
	}

	println("Start length of the list is: ", itemList.Len())

	channels := make([]chan *dynamicList.Item, 3)
	for i := range channels {
		channels[i] = make(chan *dynamicList.Item, numberOfItems)
	}

	var wg sync.WaitGroup

	wg.Add(len(channels))
	for idx, channel := range channels {
		go dynamicList.PopItems(&itemList, dynamicList.IntPriority[idx], channel, &wg)
	}

	go func() {
		wg.Wait()
		for _, ch := range channels {
			close(ch)
		}
	}()

	// Print the popped items
	for idx, channel := range channels {
		dynamicList.PrintPoppedItems(channel, dynamicList.IntPriority[idx])
	}

	println("The process took: ", time.Since(startTime).String())
	println("End length of the list is: ", itemList.Len())
}
