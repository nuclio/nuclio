// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"io"
	"io/ioutil"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/errors"
)

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     bitReader // Input source
	toRead []byte    // Uncompressed data ready to be emitted from Read
	blkLen int       // Uncompressed bytes left to read in meta-block
	insLen int       // Bytes left to insert in current command
	cpyLen int       // Bytes left to copy in current command
	last   bool      // Last block bit detected
	err    error     // Persistent error

	step      func(*Reader) // Single step of decompression work (can panic)
	stepState int           // The sub-step state for certain steps

	mtf     internal.MoveToFront // Local move-to-front decoder
	dict    dictDecoder          // Dynamic sliding dictionary
	iacBlk  blockDecoder         // Insert-and-copy block decoder
	litBlk  blockDecoder         // Literal block decoder
	distBlk blockDecoder         // Distance block decoder

	// Literal decoding state fields.
	litMapType []uint8 // The current literal context map for the current block type
	litMap     []uint8 // Literal context map
	cmode      uint8   // The current context mode
	cmodes     []uint8 // Literal context modes

	// Distance decoding state fields.
	distMap     []uint8 // Distance context map
	distMapType []uint8 // The current distance context map for the current block type
	dist        int     // The current distance (may not be in dists)
	dists       [4]int  // Last few distances (newest-to-oldest)
	distZero    bool    // Implicit zero distance symbol found
	npostfix    uint8   // Postfix bits used in distance decoding
	ndirect     uint8   // Number of direct distance codes

	// Static dictionary state fields.
	word    []byte            // Transformed word obtained from static dictionary
	wordBuf [maxWordSize]byte // Buffer to write a transformed word into

	// Meta data fields.
	metaRd  io.LimitedReader // Local LimitedReader to reduce allocation
	metaWr  io.Writer        // Writer to write meta data to
	metaBuf []byte           // Scratch space for reading meta data
}

type blockDecoder struct {
	numTypes int             // Total number of types
	typeLen  int             // The number of blocks left for this type
	types    [2]uint8        // The current (0) and previous (1) block type
	decType  prefixDecoder   // Prefix decoder for the type symbol
	decLen   prefixDecoder   // Prefix decoder for block length
	prefixes []prefixDecoder // Prefix decoders for each block type
}

type ReaderConfig struct {
	_ struct{} // Blank field to prevent unkeyed struct literals
}

func NewReader(r io.Reader, conf *ReaderConfig) (*Reader, error) {
	br := new(Reader)
	br.Reset(r)
	return br, nil
}

func (br *Reader) Read(buf []byte) (int, error) {
	for {
		if len(br.toRead) > 0 {
			cnt := copy(buf, br.toRead)
			br.toRead = br.toRead[cnt:]
			br.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if br.err != nil {
			return 0, br.err
		}

		// Perform next step in decompression process.
		br.rd.offset = br.InputOffset
		func() {
			defer errors.Recover(&br.err)
			br.step(br)
		}()
		br.InputOffset = br.rd.FlushOffset()
		if br.err != nil {
			br.toRead = br.dict.ReadFlush() // Flush what's left in case of error
		}
	}
}

func (br *Reader) Close() error {
	if br.err == io.EOF || br.err == io.ErrClosedPipe {
		br.toRead = nil // Make sure future reads fail
		br.err = io.ErrClosedPipe
		return nil
	}
	return br.err // Return the persistent error
}

func (br *Reader) Reset(r io.Reader) error {
	*br = Reader{
		rd:   br.rd,
		step: (*Reader).readStreamHeader,

		dict:    br.dict,
		iacBlk:  br.iacBlk,
		litBlk:  br.litBlk,
		distBlk: br.distBlk,
		word:    br.word[:0],
		cmodes:  br.cmodes[:0],
		litMap:  br.litMap[:0],
		distMap: br.distMap[:0],
		dists:   [4]int{4, 11, 15, 16}, // RFC section 4

		// TODO(dsnet): Should we write meta data somewhere useful?
		metaWr:  ioutil.Discard,
		metaBuf: br.metaBuf,
	}
	br.rd.Init(r)
	return nil
}

// readStreamHeader reads the Brotli stream header according to RFC section 9.1.
func (br *Reader) readStreamHeader() {
	wbits := br.rd.ReadSymbol(&decWinBits)
	if wbits == 0 {
		errors.Panic(errCorrupted) // Reserved value used
	}
	size := int(1<<wbits) - 16
	br.dict.Init(size)
	br.readBlockHeader()
}

// readBlockHeader reads a meta-block header according to RFC section 9.2.
func (br *Reader) readBlockHeader() {
	if br.last {
		if br.rd.ReadPads() > 0 {
			errors.Panic(errCorrupted)
		}
		errors.Panic(io.EOF)
	}

	// Read ISLAST and ISLASTEMPTY.
	if br.last = br.rd.ReadBits(1) == 1; br.last {
		if empty := br.rd.ReadBits(1) == 1; empty {
			br.readBlockHeader() // Next call will terminate stream
			return
		}
	}

	// Read MLEN and MNIBBLES and process meta data.
	var blkLen int // 1..1<<24
	nibbles := br.rd.ReadBits(2) + 4
	if nibbles == 7 {
		if reserved := br.rd.ReadBits(1) == 1; reserved {
			errors.Panic(errCorrupted)
		}

		var skipLen int // 0..1<<24
		if skipBytes := br.rd.ReadBits(2); skipBytes > 0 {
			skipLen = int(br.rd.ReadBits(skipBytes * 8))
			if skipBytes > 1 && skipLen>>((skipBytes-1)*8) == 0 {
				errors.Panic(errCorrupted) // Shortest representation not used
			}
			skipLen++
		}

		if br.rd.ReadPads() > 0 {
			errors.Panic(errCorrupted)
		}
		br.blkLen = skipLen // Use blkLen to track metadata number of bytes
		br.readMetaData()
		return
	}
	blkLen = int(br.rd.ReadBits(nibbles * 4))
	if nibbles > 4 && blkLen>>((nibbles-1)*4) == 0 {
		errors.Panic(errCorrupted) // Shortest representation not used
	}
	br.blkLen = blkLen + 1

	// Read ISUNCOMPRESSED and process uncompressed data.
	if !br.last {
		if uncompressed := br.rd.ReadBits(1) == 1; uncompressed {
			if br.rd.ReadPads() > 0 {
				errors.Panic(errCorrupted)
			}
			br.readRawData()
			return
		}
	}
	br.readPrefixCodes()
}

// readMetaData reads meta data according to RFC section 9.2.
func (br *Reader) readMetaData() {
	br.metaRd.R = &br.rd
	br.metaRd.N = int64(br.blkLen)
	if br.metaBuf == nil {
		br.metaBuf = make([]byte, 4096) // Lazy allocate
	}
	if cnt, err := io.CopyBuffer(br.metaWr, &br.metaRd, br.metaBuf); err != nil {
		errors.Panic(err) // Will never panic with io.EOF
	} else if cnt < int64(br.blkLen) {
		errors.Panic(io.ErrUnexpectedEOF)
	}
	br.step = (*Reader).readBlockHeader
}

// readRawData reads raw data according to RFC section 9.2.
func (br *Reader) readRawData() {
	buf := br.dict.WriteSlice()
	if len(buf) > br.blkLen {
		buf = buf[:br.blkLen]
	}

	cnt, err := br.rd.Read(buf)
	br.blkLen -= cnt
	br.dict.WriteMark(cnt)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		errors.Panic(err)
	}

	if br.blkLen > 0 {
		br.toRead = br.dict.ReadFlush()
		br.step = (*Reader).readRawData // We need to continue this work
		return
	}
	br.step = (*Reader).readBlockHeader
}

// readPrefixCodes reads the prefix codes according to RFC section 9.2.
func (br *Reader) readPrefixCodes() {
	// Read block types for literal, insert-and-copy, and distance blocks.
	for _, bd := range []*blockDecoder{&br.litBlk, &br.iacBlk, &br.distBlk} {
		// Note: According to RFC section 6, it is okay for the block count to
		// *not* count down to zero. Thus, there is no need to validate that
		// typeLen is within some reasonable range.
		bd.types = [2]uint8{0, 1}
		bd.typeLen = -1 // Stay on this type until next meta-block

		bd.numTypes = int(br.rd.ReadSymbol(&decCounts)) // 1..256
		if bd.numTypes >= 2 {
			br.rd.ReadPrefixCode(&bd.decType, uint(bd.numTypes)+2)
			br.rd.ReadPrefixCode(&bd.decLen, uint(numBlkCntSyms))
			sym := br.rd.ReadSymbol(&bd.decLen)
			bd.typeLen = int(br.rd.ReadOffset(sym, blkLenRanges))
		}
	}

	// Read NPOSTFIX and NDIRECT.
	npostfix := br.rd.ReadBits(2)            // 0..3
	ndirect := br.rd.ReadBits(4) << npostfix // 0..120
	br.npostfix, br.ndirect = uint8(npostfix), uint8(ndirect)
	numDistSyms := 16 + ndirect + 48<<npostfix

	// Read CMODE, the literal context modes.
	br.cmodes = allocUint8s(br.cmodes, br.litBlk.numTypes)
	for i := range br.cmodes {
		br.cmodes[i] = uint8(br.rd.ReadBits(2))
	}
	br.cmode = br.cmodes[0] // 0..3

	// Read CMAPL, the literal context map.
	numLitTrees := int(br.rd.ReadSymbol(&decCounts)) // 1..256
	br.litMap = allocUint8s(br.litMap, maxLitContextIDs*br.litBlk.numTypes)
	if numLitTrees >= 2 {
		br.readContextMap(br.litMap, uint(numLitTrees))
	} else {
		for i := range br.litMap {
			br.litMap[i] = 0
		}
	}
	br.litMapType = br.litMap[0:] // First block type is zero

	// Read CMAPD, the distance context map.
	numDistTrees := int(br.rd.ReadSymbol(&decCounts)) // 1..256
	br.distMap = allocUint8s(br.distMap, maxDistContextIDs*br.distBlk.numTypes)
	if numDistTrees >= 2 {
		br.readContextMap(br.distMap, uint(numDistTrees))
	} else {
		for i := range br.distMap {
			br.distMap[i] = 0
		}
	}
	br.distMapType = br.distMap[0:] // First block type is zero

	// Read HTREEL[], HTREEI[], and HTREED[], the arrays of prefix codes.
	br.litBlk.prefixes = extendDecoders(br.litBlk.prefixes, numLitTrees)
	for i := range br.litBlk.prefixes {
		br.rd.ReadPrefixCode(&br.litBlk.prefixes[i], numLitSyms)
	}
	br.iacBlk.prefixes = extendDecoders(br.iacBlk.prefixes, br.iacBlk.numTypes)
	for i := range br.iacBlk.prefixes {
		br.rd.ReadPrefixCode(&br.iacBlk.prefixes[i], numIaCSyms)
	}
	br.distBlk.prefixes = extendDecoders(br.distBlk.prefixes, numDistTrees)
	for i := range br.distBlk.prefixes {
		br.rd.ReadPrefixCode(&br.distBlk.prefixes[i], numDistSyms)
	}

	br.step = (*Reader).readCommands
}

// readCommands reads block commands according to RFC section 9.3.
func (br *Reader) readCommands() {
	// Since Go does not support tail call optimization, we use goto statements
	// to achieve higher performance processing each command. Each label can be
	// thought of as a mini function, and each goto as a cheap function call.
	// The following code follows this control flow.
	//
	// The bulk of the action will be in the following loop:
	//	startCommand -> readLiterals -> readDistance -> copyDynamicDict ->
	//		finishCommand -> startCommand -> ...
	/*
		             readCommands()
		                   |
		+----------------> +
		|                  |
		|                  V
		|         +-- startCommand
		|         |        |
		|         |        V
		|         |   readLiterals ----------+
		|         |        |                 |
		|         |        V                 |
		|         +-> readDistance           |
		|                  |                 |
		|         +--------+--------+        |
		|         |                 |        |
		|         V                 V        |
		|  copyDynamicDict   copyStaticDict  |
		|         |                 |        |
		|         +--------+--------+        |
		|                  |                 |
		|                  V                 |
		+----------- finishCommand <---------+
		                   |
		                   V
		           readBlockHeader()
	*/

	const (
		stateInit = iota // Zero value must be stateInit

		// Some labels (readLiterals, copyDynamicDict, copyStaticDict) require
		// work to be continued if more buffer space is needed. This is achieved
		// by the  switch block right below, which continues the work at the
		// right label based on the given sub-step value.
		stateLiterals
		stateDynamicDict
		stateStaticDict
	)

	switch br.stepState {
	case stateInit:
		goto startCommand
	case stateLiterals:
		goto readLiterals
	case stateDynamicDict:
		goto copyDynamicDict
	case stateStaticDict:
		goto copyStaticDict
	}

startCommand:
	// Read the insert and copy lengths according to RFC section 5.
	{
		if br.iacBlk.typeLen == 0 {
			br.readBlockSwitch(&br.iacBlk)
		}
		br.iacBlk.typeLen--

		iacTree := &br.iacBlk.prefixes[br.iacBlk.types[0]]
		iacSym, ok := br.rd.TryReadSymbol(iacTree)
		if !ok {
			iacSym = br.rd.ReadSymbol(iacTree)
		}
		rec := iacLUT[iacSym]
		insExtra, ok := br.rd.TryReadBits(uint(rec.ins.bits))
		if !ok {
			insExtra = br.rd.ReadBits(uint(rec.ins.bits))
		}
		cpyExtra, ok := br.rd.TryReadBits(uint(rec.cpy.bits))
		if !ok {
			cpyExtra = br.rd.ReadBits(uint(rec.cpy.bits))
		}
		br.insLen = int(rec.ins.base) + int(insExtra)
		br.cpyLen = int(rec.cpy.base) + int(cpyExtra)
		br.distZero = iacSym < 128
		if br.insLen > 0 {
			goto readLiterals
		}
		goto readDistance
	}

readLiterals:
	// Read literal symbols as uncompressed data according to RFC section 9.3.
	{
		buf := br.dict.WriteSlice()
		if len(buf) > br.insLen {
			buf = buf[:br.insLen]
		}

		p1, p2 := br.dict.LastBytes()
		for i := range buf {
			if br.litBlk.typeLen == 0 {
				br.readBlockSwitch(&br.litBlk)
				br.litMapType = br.litMap[64*int(br.litBlk.types[0]):]
				br.cmode = br.cmodes[br.litBlk.types[0]] // 0..3
			}
			br.litBlk.typeLen--

			litCID := getLitContextID(p1, p2, br.cmode) // 0..63
			litTree := &br.litBlk.prefixes[br.litMapType[litCID]]
			litSym, ok := br.rd.TryReadSymbol(litTree)
			if !ok {
				litSym = br.rd.ReadSymbol(litTree)
			}

			buf[i] = byte(litSym)
			p1, p2 = byte(litSym), p1
			br.dict.WriteMark(1)
		}
		br.insLen -= len(buf)
		br.blkLen -= len(buf)

		if br.insLen > 0 {
			br.toRead = br.dict.ReadFlush()
			br.step = (*Reader).readCommands
			br.stepState = stateLiterals // Need to continue work here
			return
		}
		if br.blkLen > 0 {
			goto readDistance
		}
		goto finishCommand
	}

readDistance:
	// Read and decode the distance length according to RFC section 9.3.
	{
		if br.distZero {
			br.dist = br.dists[0]
		} else {
			if br.distBlk.typeLen == 0 {
				br.readBlockSwitch(&br.distBlk)
				br.distMapType = br.distMap[4*int(br.distBlk.types[0]):]
			}
			br.distBlk.typeLen--

			distCID := getDistContextID(br.cpyLen) // 0..3
			distTree := &br.distBlk.prefixes[br.distMapType[distCID]]
			distSym, ok := br.rd.TryReadSymbol(distTree)
			if !ok {
				distSym = br.rd.ReadSymbol(distTree)
			}

			if distSym < 16 { // Short-code
				rec := distShortLUT[distSym]
				br.dist = br.dists[rec.index] + rec.delta
			} else if distSym < uint(16+br.ndirect) { // Direct-code
				br.dist = int(distSym - 15) // 1..ndirect
			} else { // Long-code
				rec := distLongLUT[br.npostfix][distSym-uint(16+br.ndirect)]
				extra, ok := br.rd.TryReadBits(uint(rec.bits))
				if !ok {
					extra = br.rd.ReadBits(uint(rec.bits))
				}
				br.dist = int(br.ndirect) + int(rec.base) + int(extra<<br.npostfix)
			}
			br.distZero = bool(distSym == 0)
			if br.dist <= 0 {
				errors.Panic(errCorrupted)
			}
		}

		if br.dist <= br.dict.HistSize() {
			if !br.distZero {
				br.dists[3] = br.dists[2]
				br.dists[2] = br.dists[1]
				br.dists[1] = br.dists[0]
				br.dists[0] = br.dist
			}
			goto copyDynamicDict
		}
		goto copyStaticDict
	}

copyDynamicDict:
	// Copy a string from the past uncompressed data according to RFC section 2.
	{
		cnt := br.dict.WriteCopy(br.dist, br.cpyLen)
		br.blkLen -= cnt
		br.cpyLen -= cnt

		if br.cpyLen > 0 {
			br.toRead = br.dict.ReadFlush()
			br.step = (*Reader).readCommands
			br.stepState = stateDynamicDict // Need to continue work here
			return
		}
		goto finishCommand
	}

copyStaticDict:
	// Copy a string from the static dictionary according to RFC section 8.
	{
		if len(br.word) == 0 {
			if br.cpyLen < minDictLen || br.cpyLen > maxDictLen {
				errors.Panic(errCorrupted)
			}
			wordIdx := br.dist - (br.dict.HistSize() + 1)
			index := wordIdx % dictSizes[br.cpyLen]
			offset := dictOffsets[br.cpyLen] + index*br.cpyLen
			baseWord := dictLUT[offset : offset+br.cpyLen]
			transformIdx := wordIdx >> uint(dictBitSizes[br.cpyLen])
			if transformIdx >= len(transformLUT) {
				errors.Panic(errCorrupted)
			}
			cnt := transformWord(br.wordBuf[:], baseWord, transformIdx)
			br.word = br.wordBuf[:cnt]
		}

		buf := br.dict.WriteSlice()
		cnt := copy(buf, br.word)
		br.word = br.word[cnt:]
		br.blkLen -= cnt
		br.dict.WriteMark(cnt)

		if len(br.word) > 0 {
			br.toRead = br.dict.ReadFlush()
			br.step = (*Reader).readCommands
			br.stepState = stateStaticDict // Need to continue work here
			return
		}
		goto finishCommand
	}

finishCommand:
	// Finish off this command and check if we need to loop again.
	if br.blkLen < 0 {
		errors.Panic(errCorrupted)
	}
	if br.blkLen > 0 {
		goto startCommand // More commands in this block
	}

	// Done with this block.
	br.toRead = br.dict.ReadFlush()
	br.step = (*Reader).readBlockHeader
	br.stepState = stateInit // Next call to readCommands must start here
}

// readContextMap reads the context map according to RFC section 7.3.
func (br *Reader) readContextMap(cm []uint8, numTrees uint) {
	// TODO(dsnet): Test the following edge cases:
	// * Test with largest and smallest MAXRLE sizes
	// * Test with with very large MAXRLE value
	// * Test inverseMoveToFront

	maxRLE := br.rd.ReadSymbol(&decMaxRLE)
	br.rd.ReadPrefixCode(&br.rd.prefix, maxRLE+numTrees)
	for i := 0; i < len(cm); {
		sym := br.rd.ReadSymbol(&br.rd.prefix)
		if sym == 0 || sym > maxRLE {
			// Single non-zero value.
			if sym > 0 {
				sym -= maxRLE
			}
			cm[i] = uint8(sym)
			i++
		} else {
			// Repeated zeros.
			n := int(br.rd.ReadOffset(sym-1, maxRLERanges))
			if i+n > len(cm) {
				errors.Panic(errCorrupted)
			}
			for j := i + n; i < j; i++ {
				cm[i] = 0
			}
		}
	}

	if invert := br.rd.ReadBits(1) == 1; invert {
		br.mtf.Decode(cm)
	}
}

// readBlockSwitch handles a block switch command according to RFC section 6.
func (br *Reader) readBlockSwitch(bd *blockDecoder) {
	symType := br.rd.ReadSymbol(&bd.decType)
	switch symType {
	case 0:
		symType = uint(bd.types[1])
	case 1:
		symType = uint(bd.types[0]) + 1
		if symType >= uint(bd.numTypes) {
			symType -= uint(bd.numTypes)
		}
	default:
		symType -= 2
	}
	bd.types = [2]uint8{uint8(symType), bd.types[0]}

	symLen := br.rd.ReadSymbol(&bd.decLen)
	bd.typeLen = int(br.rd.ReadOffset(symLen, blkLenRanges))
}
