// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

// meta_stats is used to measure the efficiency of meta encoding.
//
// The XFLATE meta encoding is a block-based format that allows in-band encoding
// of arbitrary meta data into a DEFLATE stream. This is possible because when
// XFLATE meta encoding is decoded using a DEFLATE decoder, it produces zero
// output. This allows XFLATE to add information on top of DEFLATE in a
// backwards compatible manner.
//
// The meta encoding works by using dynamic DEFLATE blocks and encoding bits of
// information into the HLIT tree definition. To ensure that the meta encoding
// produces no output when decompressed, these dynamic blocks are terminated
// immediately with an end-of-block (EOB) marker. Since Huffman tree definitions
// were not designed to encode arbitrary data into them, there is some overhead
// encoding meta data into them in a way that still produces valid trees.
//
// The purpose of this program is to measure the efficiency of the meta encoding
// by computing the ratio of the raw input to the encoded output. Since the HLIT
// tree can only handle some maximum number of symbols, the number of bytes that
// can be encoded into a single dynamic block is capped at some limit.
// This program also attempts to determine what that limit is.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"math/rand"

	"github.com/dsnet/compress/xflate/internal/meta"
)

func init() { log.SetFlags(log.Lshortfile) }

func main() {
	// It would be impractical to try all possible input strings.
	// Thus, we sample random strings from the domain by performing
	// numSamples trials per size class.
	numSamples := flag.Int("n", 256, "number of strings to sample per size class")
	randSeed := flag.Int("s", 0, "random number generator seed")
	flag.Parse()

	// Compute the combination nCk.
	comb := func(n, k int64) *big.Int {
		val := big.NewInt(1)
		for j := int64(0); j < k; j++ {
			// Compute: val = (val*(n-j)) / (j+1)
			val.Div(new(big.Int).Mul(val, big.NewInt(n-j)), big.NewInt(j+1))
		}
		return val
	}

	// Convert big integer to float64.
	rtof := func(r *big.Rat) float64 { f, _ := r.Float64(); return f }
	itof := func(i *big.Int) float64 { return rtof(new(big.Rat).SetInt(i)) }

	// Print titles of each category to compute metrics on:
	//	NumBytes: The length of the input string.
	//	FullDomain: Are all possible strings of this length encodable?
	//	Coverage: What percentage of all possible strings of this length can be encoded?
	//	Efficiency: The efficiency of encoding; this compares NumBytes to EncSize.
	//	EncSize: The size of the output when encoding a string of this length.
	fmt.Println("NumBytes  FullDomain  Coverage  EncSize[min<=avg<=max]   Efficiency[max=>avg=>min]")

	var buf []byte
	mw := new(meta.Writer)
	rand := rand.New(rand.NewSource(int64(*randSeed)))
	for numBytes := meta.MinRawBytes; numBytes <= meta.MaxRawBytes; numBytes++ {
		numBits := numBytes * 8

		// Whether a string is encodable or not is entirely based on the number
		// of one bits and zero bits in the string. Thus, we gather results from
		// every possible size class.
		encodable := big.NewInt(0) // Total number of input strings that are encodable
		encMin, encMax, encTotal := math.MaxInt8, math.MinInt8, 0.0
		for zeros := 0; zeros <= numBits; zeros++ {
			ones := numBits - zeros

			// If huffLen is 0, then that means that a string with this many
			// zero bits and bits is not encodable.
			if huffLen, _ := computeHuffLen(zeros, ones); huffLen == 0 {
				continue
			}

			// The total number of unique strings with the given number of zero
			// bits and one bits is the combination nCk where n is the total
			// number of bits and k is the the total number of one bits.
			num := comb(int64(numBits), int64(ones))
			encodable.Add(encodable, num)

			// For numSamples trials, keep track of the minimum, average, and
			// maximum size of the encoded output.
			encAvg := 0.0
			for i := 0; i < *numSamples; i++ {
				// Generate a random string permutation.
				buf = buf[:0]
				perm := rand.Perm(numBits)
				for i := 0; i < numBits/8; i++ {
					var b byte
					for j := 0; j < 8; j++ {
						if perm[8*i+j] >= zeros {
							b |= 1 << uint(j)
						}
					}
					buf = append(buf, b)
				}

				// Encode the string and compute the output length.
				mw.Reset(ioutil.Discard)
				if _, err := mw.Write(buf); err != nil {
					log.Fatal(err)
				}
				if err := mw.Close(); err != nil {
					log.Fatal(err)
				}
				if mw.NumBlocks != 1 {
					log.Fatal("unexpected extra blocks")
				}

				cnt := int(mw.OutputOffset)
				if encMin > cnt {
					encMin = cnt
				}
				if encMax < cnt {
					encMax = cnt
				}
				encAvg += float64(cnt) / float64(*numSamples)
			}

			// Weighted total based on the number of strings.
			encTotal += itof(num) * encAvg
		}

		// If no input string is encodable, don't bother printing results.
		if encodable.Cmp(new(big.Int)) == 0 {
			continue
		}

		encAvg := encTotal / itof(encodable)                              // encAvg     := encTotal / encodable
		domain := new(big.Int).Lsh(big.NewInt(1), uint(numBits))          // domain     := 1 << numBits
		fullDomain := encodable.Cmp(domain) == 0                          // fullDomain := encodable == domain
		coverage := 100.0 * rtof(new(big.Rat).SetFrac(encodable, domain)) // coverage   := 100.0 * (encodable / domain)
		maxEff := 100.0 * (float64(numBytes) / float64(encMin))           // maxEff     := 100.0 *  (numBytes / encMin)
		avgEff := 100.0 * (float64(numBytes) / float64(encAvg))           // avgEff     := 100.0 *  (numBytes / encAvg)
		minEff := 100.0 * (float64(numBytes) / float64(encMax))           // minEff     := 100.0 *  (numBytes / encMax)

		fmt.Printf("%8d%12v%9.2f%%     [%2d <= %4.2f <= %2d]  [%5.1f%% => %4.1f%% => %4.1f%%]\n",
			numBytes, fullDomain, coverage, encMin, encAvg, encMax, maxEff, avgEff, minEff)
	}
}

// computeHuffLen computes the shortest Huffman length to encode the data.
// If the input data is too large, then 0 is returned.
//
// This is copied from Writer.computeHuffLen to avoid visibility issues.
func computeHuffLen(zeros, ones int) (huffLen uint, inv bool) {
	const (
		maxSyms    = 257 // Maximum number of literal codes (with EOB marker)
		minHuffLen = 1   // Minimum number of bits for each Huffman code
		maxHuffLen = 7   // Maximum number of bits for each Huffman code
	)

	if inv = ones > zeros; inv {
		zeros, ones = ones, zeros
	}
	for huffLen = minHuffLen; huffLen <= maxHuffLen; huffLen++ {
		maxOnes := 1 << uint(huffLen)
		if maxSyms-maxOnes >= zeros+8 && maxOnes >= ones+8 {
			return huffLen, inv
		}
	}
	return 0, false
}
