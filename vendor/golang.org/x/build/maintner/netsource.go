// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/build/maintner/maintpb"
	"golang.org/x/build/maintner/reclog"
)

// NewNetworkMutationSource returns a mutation source from a master server.
// The server argument should be a URL to the JSON logs index.
func NewNetworkMutationSource(server, cacheDir string) MutationSource {
	base, err := url.Parse(server)
	if err != nil {
		panic(fmt.Sprintf("invalid URL: %q", server))
	}
	return &netMutSource{
		server:   server,
		base:     base,
		cacheDir: cacheDir,
	}
}

type netMutSource struct {
	server   string
	base     *url.URL
	cacheDir string

	last []fileSeg

	// Hooks for testing. If nil, unused:
	testHookGetServerSegments      func(context.Context, int64) ([]LogSegmentJSON, error)
	testHookWaitAfterServerDupData func(context.Context) error
	testHookSyncSeg                func(context.Context, LogSegmentJSON) (fileSeg, error)
	testHookFilePrefixSum224       func(file string, n int64) string
}

func (ns *netMutSource) GetMutations(ctx context.Context) <-chan MutationStreamEvent {
	ch := make(chan MutationStreamEvent, 50)
	go func() {
		err := ns.sendMutations(ctx, ch)
		final := MutationStreamEvent{Err: err}
		if err == nil {
			final.End = true
		}
		select {
		case ch <- final:
		case <-ctx.Done():
		}
	}()
	return ch
}

// isNoInternetError reports whether the provided error is because there's no
// network connectivity.
func isNoInternetError(err error) bool {
	if err == nil {
		return false
	}
	switch err := err.(type) {
	case *url.Error:
		return isNoInternetError(err.Err)
	case *net.OpError:
		return isNoInternetError(err.Err)
	case *net.DNSError:
		// Trashy:
		return err.Err == "no such host"
	default:
		log.Printf("Unknown error type %T: %#v", err, err)
		return false
	}
}

func (ns *netMutSource) locallyCachedSegments() (segs []fileSeg, err error) {
	defer func() {
		if err != nil {
			log.Printf("No network connection and failed to use local cache: %v", err)
		} else {
			log.Printf("No network connection; using %d locally cached segments.", len(segs))
		}
	}()
	fis, err := ioutil.ReadDir(ns.cacheDir)
	if err != nil {
		return nil, err
	}
	fiMap := map[string]os.FileInfo{}
	segHex := map[int]string{}
	segGrowing := map[int]bool{}
	for _, fi := range fis {
		name := fi.Name()
		if !strings.HasSuffix(name, ".mutlog") {
			continue
		}
		fiMap[name] = fi

		if len(name) == len("0000.6897fab4d3afcda332424b2a2a1a4469021074282bc7be5606aaa221.mutlog") {
			num, err := strconv.Atoi(name[:4])
			if err != nil {
				continue
			}
			segHex[num] = strings.TrimSuffix(name[5:], ".mutlog")
		} else if strings.HasSuffix(name, ".growing.mutlog") {
			num, err := strconv.Atoi(name[:4])
			if err != nil {
				continue
			}
			segGrowing[num] = true
		}
	}
	for num := 0; ; num++ {
		if hex, ok := segHex[num]; ok {
			name := fmt.Sprintf("%04d.%s.mutlog", num, hex)
			segs = append(segs, fileSeg{
				seg:    num,
				file:   filepath.Join(ns.cacheDir, name),
				size:   fiMap[name].Size(),
				sha224: hex,
			})
			continue
		}
		if segGrowing[num] {
			name := fmt.Sprintf("%04d.growing.mutlog", num)
			slurp, err := ioutil.ReadFile(filepath.Join(ns.cacheDir, name))
			if err != nil {
				return nil, err
			}
			segs = append(segs, fileSeg{
				seg:    num,
				file:   filepath.Join(ns.cacheDir, name),
				size:   int64(len(slurp)),
				sha224: fmt.Sprintf("%x", sha256.Sum224(slurp)),
			})
		}
		return segs, nil
	}
}

// waitSizeNot optionally specifies that the request should long-poll waiting for the server
// to have a sum of log segment sizes different than the value specified.
func (ns *netMutSource) getServerSegments(ctx context.Context, waitSizeNot int64) ([]LogSegmentJSON, error) {
	if fn := ns.testHookGetServerSegments; fn != nil {
		return fn(ctx, waitSizeNot)
	}

	logsURL := ns.server
	if waitSizeNot > 0 {
		logsURL += fmt.Sprintf("?waitsizenot=%d", waitSizeNot)
	}
	for {
		req, err := http.NewRequest("GET", logsURL, nil)
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		// If we're doing a long poll and the server replies
		// with a 304 response, that means the server is just
		// heart-beating us and trying to get a response back
		// within its various deadlines. But we should just
		// try again.
		if waitSizeNot > 0 && res.StatusCode == http.StatusNotModified {
			res.Body.Close()
			continue
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("%s: %v", ns.server, res.Status)
		}
		var segs []LogSegmentJSON
		err = json.NewDecoder(res.Body).Decode(&segs)
		if err != nil {
			return nil, fmt.Errorf("decoding %s JSON: %v", ns.server, err)
		}
		return segs, nil
	}
}

func (ns *netMutSource) getNewSegments(ctx context.Context) ([]fileSeg, error) {
	for {
		sumLast := sumSegSize(ns.last)

		segs, err := ns.getServerSegments(ctx, sumLast)
		if isNoInternetError(err) {
			if sumLast == 0 {
				return ns.locallyCachedSegments()
			}
			log.Printf("No internet; blocking.")
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(15 * time.Second):
				continue
			}
		}
		if err != nil {
			return nil, err
		}
		// TODO: optimization: if already on GCE, skip sync to disk part and just
		// read from network. fast & free network inside.

		var fileSegs []fileSeg
		for _, seg := range segs {
			fileSeg, err := ns.syncSeg(ctx, seg)
			if err != nil {
				return nil, fmt.Errorf("syncing segment %d: %v", seg.Number, err)
			}
			fileSegs = append(fileSegs, fileSeg)
		}
		sumCommon := ns.sumCommonPrefixSize(fileSegs, ns.last)
		if sumLast != sumCommon {
			return nil, ErrSplit
		}
		sumCur := sumSegSize(fileSegs)
		if sumCommon == sumCur {
			// Nothing new. This shouldn't happen once the
			// server is updated to respect the
			// "?waitsizenot=NNN" long polling parameter.
			// But keep this brief pause as a backup to
			// prevent spinning and because clients &
			// servers won't be updated simultaneously.
			if ns.testHookGetServerSegments == nil {
				log.Printf("maintner.netsource: server returned unchanged log segments; old server?")
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
			}
			continue
		}
		ns.last = fileSegs

		newSegs := trimLeadingSegBytes(fileSegs, sumCommon)
		return newSegs, nil
	}
}

func trimLeadingSegBytes(in []fileSeg, trim int64) []fileSeg {
	// First trim off whole segments, sharing the same underlying memory.
	for len(in) > 0 && trim >= in[0].size {
		trim -= in[0].size
		in = in[1:]
	}
	if len(in) == 0 {
		return nil
	}
	// Now copy, since we'll be modifying the first element.
	out := append([]fileSeg(nil), in...)
	out[0].skip = trim
	return out
}

// filePrefixSum224 returns the lowercase hex SHA-224 of the first n bytes of file.
func (ns *netMutSource) filePrefixSum224(file string, n int64) string {
	if fn := ns.testHookFilePrefixSum224; fn != nil {
		return fn(file, n)
	}
	f, err := os.Open(file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Print(err)
		}
		return ""
	}
	defer f.Close()
	h := sha256.New224()
	_, err = io.CopyN(h, f, n)
	if err != nil {
		log.Print(err)
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func sumSegSize(segs []fileSeg) (sum int64) {
	for _, seg := range segs {
		sum += seg.size
	}
	return
}

func (ns *netMutSource) sumCommonPrefixSize(a, b []fileSeg) (sum int64) {
	for len(a) > 0 && len(b) > 0 {
		sa, sb := a[0], b[0]
		if sa.sha224 == sb.sha224 {
			// Whole chunk in common.
			sum += sa.size
			a, b = a[1:], b[1:]
			continue
		}
		if sa.size == sb.size {
			// If they're the same size but different
			// sums, it must've forked.
			return
		}
		// See if one chunk is a prefix of the other.
		// Make sa be the smaller one.
		if sb.size < sa.size {
			sa, sb = sb, sa
		}
		// Hash the beginning of the bigger size.
		bPrefixSum := ns.filePrefixSum224(sb.file, sa.size)
		if bPrefixSum == sa.sha224 {
			sum += sa.size
		}
		break
	}
	return
}

func (ns *netMutSource) sendMutations(ctx context.Context, ch chan<- MutationStreamEvent) error {
	newSegs, err := ns.getNewSegments(ctx)
	if err != nil {
		return err
	}
	return foreachFileSeg(newSegs, func(seg fileSeg) error {
		f, err := os.Open(seg.file)
		if err != nil {
			return err
		}
		defer f.Close()
		if seg.skip > 0 {
			if _, err := f.Seek(seg.skip, io.SeekStart); err != nil {
				return err
			}
		}
		return reclog.ForeachRecord(io.LimitReader(f, seg.size-seg.skip), seg.skip, func(off int64, hdr, rec []byte) error {
			m := new(maintpb.Mutation)
			if err := proto.Unmarshal(rec, m); err != nil {
				return err
			}
			select {
			case ch <- MutationStreamEvent{Mutation: m}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
}

func foreachFileSeg(segs []fileSeg, fn func(seg fileSeg) error) error {
	for _, seg := range segs {
		if err := fn(seg); err != nil {
			return err
		}
	}
	return nil
}

// TODO: add a constructor for this? or simplify it. make it Size +
// File + embedded LogSegmentJSON?
type fileSeg struct {
	seg    int
	file   string // full path
	sha224 string
	skip   int64
	size   int64
}

func (ns *netMutSource) syncSeg(ctx context.Context, seg LogSegmentJSON) (fileSeg, error) {
	if fn := ns.testHookSyncSeg; fn != nil {
		return fn(ctx, seg)
	}

	isFinalSeg := !strings.HasPrefix(seg.URL, "https://storage.googleapis.com/")
	relURL, err := url.Parse(seg.URL)
	if err != nil {
		return fileSeg{}, err
	}
	segURL := ns.base.ResolveReference(relURL)

	frozen := filepath.Join(ns.cacheDir, fmt.Sprintf("%04d.%s.mutlog", seg.Number, seg.SHA224))

	// Do we already have it? Files named in their final form with the sha224 are considered
	// complete and immutable.
	if fi, err := os.Stat(frozen); err == nil && fi.Size() == seg.Size {
		return fileSeg{seg: seg.Number, file: frozen, size: fi.Size(), sha224: seg.SHA224}, nil
	}

	// See how much data we already have in the partial growing file.
	partial := filepath.Join(ns.cacheDir, fmt.Sprintf("%04d.growing.mutlog", seg.Number))
	have, _ := ioutil.ReadFile(partial)
	if int64(len(have)) == seg.Size {
		got224 := fmt.Sprintf("%x", sha256.Sum224(have))
		if got224 == seg.SHA224 {
			if !isFinalSeg {
				// This was growing for us, but the server started a new growing segment.
				if err := os.Rename(partial, frozen); err != nil {
					return fileSeg{}, err
				}
				return fileSeg{seg: seg.Number, file: frozen, sha224: seg.SHA224, size: seg.Size}, nil
			}
			return fileSeg{seg: seg.Number, file: partial, sha224: seg.SHA224, size: seg.Size}, nil
		}
	}

	// Otherwise, download.
	req, err := http.NewRequest("GET", segURL.String(), nil)
	if err != nil {
		return fileSeg{}, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", len(have), seg.Size-1))

	log.Printf("Downloading %d bytes of %s ...", seg.Size-int64(len(have)), segURL)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fileSeg{}, err
	}
	if res.StatusCode != 200 && res.StatusCode != 206 {
		return fileSeg{}, fmt.Errorf("%s: %s", segURL.String(), res.Status)
	}
	slurp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return fileSeg{}, err
	}

	var newContents []byte
	if int64(len(slurp)) == seg.Size {
		newContents = slurp
	} else if int64(len(have)+len(slurp)) == seg.Size {
		newContents = append(have, slurp...)
	}
	got224 := fmt.Sprintf("%x", sha256.Sum224(newContents))
	if got224 != seg.SHA224 {
		if len(have) == 0 {
			return fileSeg{}, errors.New("corrupt download")
		}
		// Try again
		os.Remove(partial)
		return ns.syncSeg(ctx, seg)
	}
	tf, err := ioutil.TempFile(ns.cacheDir, "tempseg")
	if err != nil {
		return fileSeg{}, err
	}
	if _, err := tf.Write(newContents); err != nil {
		return fileSeg{}, err
	}
	if err := tf.Close(); err != nil {
		return fileSeg{}, err
	}
	finalName := partial
	if !isFinalSeg {
		finalName = frozen
	}
	if err := os.Rename(tf.Name(), finalName); err != nil {
		return fileSeg{}, err
	}
	log.Printf("wrote %v", finalName)
	return fileSeg{seg: seg.Number, file: finalName, size: seg.Size, sha224: seg.SHA224}, nil
}

type LogSegmentJSON struct {
	Number int    `json:"number"`
	Size   int64  `json:"size"`
	SHA224 string `json:"sha224"`
	URL    string `json:"url"`
}
