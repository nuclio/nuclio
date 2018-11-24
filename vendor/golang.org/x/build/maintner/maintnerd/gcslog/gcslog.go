// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gcslog is an implementation of maintner.MutationSource and Logger for Google Cloud Storage.
package gcslog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/golang/protobuf/proto"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/maintpb"
	"golang.org/x/build/maintner/reclog"
	"google.golang.org/api/iterator"
)

// targetObjectSize is the goal maximum size for each log segment on
// GCS. In the unlikely case that a single record is larger than this
// then a segment would be bigger than this (records are never split
// between log segments).  But otherwise this is the max size.
const targetObjectSize = 16 << 20

const flushInterval = 10 * time.Minute

// GCSLog implements MutationLogger and MutationSource.
var _ maintner.MutationLogger = &GCSLog{}
var _ maintner.MutationSource = &GCSLog{}

// GCSLog logs mutations to GCS.
type GCSLog struct {
	sc         *storage.Client
	bucketName string
	bucket     *storage.BucketHandle

	mu         sync.Mutex // guards the following
	cond       *sync.Cond
	seg        map[int]gcsLogSegment
	curNum     int
	logBuf     bytes.Buffer
	logSHA224  hash.Hash
	flushTimer *time.Timer // non-nil if flush timer is active
}

type gcsLogSegment struct {
	num     int // starting with 0
	size    int64
	sha224  string // in lowercase hex
	created time.Time
}

func (s gcsLogSegment) ObjectName() string {
	return fmt.Sprintf("%04d.%s.mutlog", s.num, s.sha224)
}

func (s gcsLogSegment) String() string {
	return fmt.Sprintf("{gcsLogSegment num=%v, size=%v, sha=%v, created=%v}", s.num, s.size, s.sha224, s.created.Format(time.RFC3339))
}

// newGCSLogBase returns a new gcsLog instance without any association
// with Google Cloud Storage.
func newGCSLogBase() *GCSLog {
	gl := &GCSLog{
		seg: map[int]gcsLogSegment{},
	}
	gl.cond = sync.NewCond(&gl.mu)
	return gl
}

// NewGCSLog creates a GCSLog that logs mutations to a given GCS bucket.
func NewGCSLog(ctx context.Context, bucketName string) (*GCSLog, error) {
	sc, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	gl := newGCSLogBase()
	gl.sc = sc
	gl.bucketName = bucketName
	gl.bucket = sc.Bucket(bucketName)
	if err := gl.initLoad(ctx); err != nil {
		return nil, err
	}
	return gl, nil
}

var objnameRx = regexp.MustCompile(`^(\d{4})\.([0-9a-f]{56})\.mutlog$`)

func (gl *GCSLog) initLoad(ctx context.Context) error {
	it := gl.bucket.Objects(ctx, nil)
	maxNum := 0
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("iterating over %s bucket: %v", gl.bucketName, err)
		}
		m := objnameRx.FindStringSubmatch(objAttrs.Name)
		if m == nil {
			log.Printf("Ignoring unrecognized GCS object %q", objAttrs.Name)
			continue
		}
		n, _ := strconv.ParseInt(m[1], 10, 32)
		seg := gcsLogSegment{
			num:     int(n),
			sha224:  m[2],
			size:    objAttrs.Size,
			created: objAttrs.Created,
		}
		if seg.num > maxNum {
			maxNum = seg.num
		}
		if prevSeg, ok := gl.seg[int(n)]; !ok || prevSeg.created.Before(seg.created) {
			gl.seg[int(n)] = seg
			log.Printf("seg[%v] = %s", n, seg)
			if ok {
				gl.deleteOldSegment(ctx, prevSeg.ObjectName())
			}
		}
	}
	gl.curNum = maxNum

	if len(gl.seg) == 0 {
		return nil
	}

	// Check for any missing segments.
	for i := 0; i < maxNum; i++ {
		if _, ok := gl.seg[i]; !ok {
			return fmt.Errorf("saw max segment number %d but missing segment %d", maxNum, i)
		}
	}

	// Should we resume writing to the latest entry?
	// If the latest one is big enough, leave it be.
	// Otherwise slurp it in and we'll append to it.
	if gl.seg[maxNum].size >= targetObjectSize-(4<<10) {
		gl.curNum++
		return nil
	}

	r, err := gl.bucket.Object(gl.seg[maxNum].ObjectName()).NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err = io.Copy(gcsLogWriter{gl}, r); err != nil {
		return err
	}

	return nil
}

func (gl *GCSLog) serveLogFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "bad method", http.StatusBadRequest)
		return
	}

	num, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/logs/"))
	if err != nil {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	gl.mu.Lock()
	if num > gl.curNum {
		gl.mu.Unlock()
		http.Error(w, "bad segment number", http.StatusBadRequest)
		return
	}
	if num != gl.curNum {
		obj := gl.seg[num].ObjectName()
		gl.mu.Unlock()
		http.Redirect(w, r, "https://storage.googleapis.com/"+gl.bucketName+"/"+obj, http.StatusFound)
		return
	}
	content := gl.logBuf.String()

	gl.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(content))
}

func (gl *GCSLog) serveJSONLogsIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "bad method", http.StatusBadRequest)
		return
	}

	startSeg, _ := strconv.Atoi(r.FormValue("startseg"))
	if startSeg < 0 {
		http.Error(w, "bad startseg", http.StatusBadRequest)
		return
	}

	// Long poll if request contains non-zero waitsizenot parameter.
	// The client's provided 'waitsizenot' value is the sum of the segment
	// sizes they already know. They're waiting for something new.
	if s := r.FormValue("waitsizenot"); s != "" {
		oldSize, err := strconv.ParseInt(s, 10, 64)
		if err != nil || oldSize < 0 {
			http.Error(w, "bad waitsizenot", http.StatusBadRequest)
			return
		}
		// Return a 304 if there's no activity in just under a minute.
		// This keeps some occasional activity on the TCP connection
		// so we (and any proxies) know it's alive, and can fit
		// within reason read/write deadlines on either side.
		ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
		defer cancel()
		changed := gl.waitSizeNot(ctx, oldSize)
		if !changed {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	segs := gl.getJSONLogs(startSeg)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Sum-Segment-Size", fmt.Sprint(sumSegmentSizes(segs)))

	body, _ := json.MarshalIndent(segs, "", "\t")
	w.Write(body)
}

// sumSegmentSizes returns the sum of each seg.Size in segs.
func sumSegmentSizes(segs []maintner.LogSegmentJSON) (sum int64) {
	for _, seg := range segs {
		sum += seg.Size
	}
	return sum
}

// waitSizeNot blocks until the sum of GCSLog is not v, or the context expires.
// It reports whether the size changed.
func (gl *GCSLog) waitSizeNot(ctx context.Context, v int64) (changed bool) {
	returned := make(chan struct{})
	defer close(returned)
	go gl.waitSizeNotAwaitContextOrChange(ctx, returned)
	gl.mu.Lock()
	defer gl.mu.Unlock()
	for {
		if curSize := gl.sumSizeLocked(); curSize != v {
			log.Printf("waitSize fired. from %d => %d", v, curSize)
			return true
		}
		select {
		case <-ctx.Done():
			return false
		default:
			gl.cond.Wait()
		}
	}
}

// waitSizeNotAwaitContextOrChange is part of waitSizeNot.
// It's a goroutine that selects on two channels and calls
// sync.Cond.Broadcast to wake up the waitSizeNot waiter if the
// context expires.
func (gl *GCSLog) waitSizeNotAwaitContextOrChange(ctx context.Context, returned <-chan struct{}) {
	select {
	case <-ctx.Done():
		gl.cond.Broadcast()
	case <-returned:
		// No need to do a wakeup. Caller is already gone.
	}
}

func (gl *GCSLog) sumSizeLocked() int64 {
	var sum int64
	for n, seg := range gl.seg {
		if n != gl.curNum {
			sum += seg.size
		}
	}
	sum += int64(gl.logBuf.Len())
	return sum
}

func (gl *GCSLog) getJSONLogs(startSeg int) (segs []maintner.LogSegmentJSON) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	if startSeg > gl.curNum || startSeg < 0 {
		startSeg = 0
	}
	segs = make([]maintner.LogSegmentJSON, 0, gl.curNum-startSeg)
	for i := startSeg; i < gl.curNum; i++ {
		seg := gl.seg[i]
		segs = append(segs, maintner.LogSegmentJSON{
			Number: i,
			Size:   seg.size,
			SHA224: seg.sha224,
			URL:    fmt.Sprintf("https://storage.googleapis.com/%s/%s", gl.bucketName, seg.ObjectName()),
		})
	}
	if gl.logBuf.Len() > 0 {
		segs = append(segs, maintner.LogSegmentJSON{
			Number: gl.curNum,
			Size:   int64(gl.logBuf.Len()),
			SHA224: fmt.Sprintf("%x", gl.logSHA224.Sum(nil)),
			URL:    fmt.Sprintf("/logs/%d", gl.curNum),
		})
	}
	return
}

// gcsLogWriter is the io.Writer used to write to GCSLog.logBuf. It
// keeps the sha224 in sync. Caller must hold gl.mu.
type gcsLogWriter struct{ gl *GCSLog }

func (w gcsLogWriter) Write(p []byte) (n int, err error) {
	gl := w.gl
	if gl.logBuf.Len() == 0 {
		gl.logSHA224 = sha256.New224()
	}
	n, err = gl.logSHA224.Write(p)
	if n != len(p) || err != nil {
		panic(fmt.Sprintf("unexpected write (%v, %v) for %v bytes", n, err, len(p)))
	}
	n, err = gl.logBuf.Write(p)
	if n != len(p) || err != nil {
		panic(fmt.Sprintf("unexpected write (%v, %v) for %v bytes", n, err, len(p)))
	}
	return len(p), nil
}

// Log writes m to GCS after the buffer is full or after a periodic flush.
func (gl *GCSLog) Log(m *maintpb.Mutation) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	gl.mu.Lock()
	defer gl.mu.Unlock()

	// If we have some data and this item would push us over, flush.
	if gl.logBuf.Len()+len(data) > targetObjectSize {
		log.Printf("Log: record requires buffer flush.")
		if err := gl.flushLocked(context.TODO()); err != nil {
			return err
		}
		gl.curNum++
		gl.logBuf.Reset()
		log.Printf("cur log file now %d", gl.curNum)
	}

	if err := reclog.WriteRecord(gcsLogWriter{gl}, int64(gl.logBuf.Len()), data); err != nil {
		return err
	}
	gl.cond.Broadcast() // wake any long-polling subscribers

	// Otherwise schedule a periodic flush.
	if gl.flushTimer == nil {
		log.Printf("wrote record; flush timer registered.")
		gl.flushTimer = time.AfterFunc(flushInterval, gl.onFlushTimer)
	} else {
		log.Printf("wrote record; using existing flush timer.")
	}
	return nil
}

func (gl *GCSLog) onFlushTimer() {
	log.Printf("flush timer fired.")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	gl.flush(ctx)
}

func (gl *GCSLog) flush(ctx context.Context) error {
	gl.mu.Lock()

	defer gl.mu.Unlock()
	if gl.flushTimer != nil {
		gl.flushTimer.Stop()
		gl.flushTimer = nil
	}

	if err := gl.flushLocked(ctx); err != nil {
		gl.flushTimer = time.AfterFunc(1*time.Minute, gl.onFlushTimer)
		log.Printf("Error background flushing: %v", err)
		return err
	}

	return nil
}

func (gl *GCSLog) flushLocked(ctx context.Context) error {
	buf := gl.logBuf.Bytes()
	if len(buf) == 0 {
		return nil
	}
	seg := gcsLogSegment{
		num:    gl.curNum,
		sha224: fmt.Sprintf("%x", sha256.Sum224(buf)),
		size:   int64(len(buf)),
	}
	objName := seg.ObjectName()
	log.Printf("flushing %s (%d bytes)", objName, len(buf))
	err := try(4, time.Second, func() error {
		w := gl.bucket.Object(objName).NewWriter(ctx)
		w.ContentType = "application/octet-stream"
		w.ACL = append(w.ACL, storage.ACLRule{Entity: storage.AllUsers, Role: storage.RoleReader})
		if _, err := w.Write(buf); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		attrs := w.Attrs()
		seg.created = attrs.Created
		return nil
	})
	if err != nil {
		return err
	}

	old := gl.seg[seg.num]
	gl.seg[seg.num] = seg

	// Atomically update the manifest file. If we lose the CAS
	// race, that means some other instance of this process is
	// running at the same time (e.g. accidental replica count > 1
	// in k8s config) and we should fail hard.
	// TODO: that^

	// Delete any old segment from the same position.
	if old.sha224 != "" && old.sha224 != seg.sha224 {
		gl.deleteOldSegment(ctx, old.ObjectName())
	}
	return nil
}

func (gl *GCSLog) deleteOldSegment(ctx context.Context, objName string) {
	err := gl.bucket.Object(objName).Delete(ctx)
	if err != nil {
		// Can ignore, though. Probably emphemeral, and not critical.
		// It'll be deleted by new versions or next start-up anyway.
		log.Printf("Warning: error deleting old segment version %v: %v", objName, err)
	} else {
		log.Printf("deleted old segment version %v", objName)
	}
}

func (gl *GCSLog) objectNames() (names []string) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	for _, seg := range gl.seg {
		names = append(names, seg.ObjectName())
	}
	sort.Strings(names)
	return
}

func (gl *GCSLog) foreachSegmentReader(ctx context.Context, fn func(r io.Reader) error) error {
	objs := gl.objectNames()
	for i, obj := range objs {
		log.Printf("Reading %d/%d: %s ...", i+1, len(objs), obj)
		rd, err := gl.bucket.Object(obj).NewReader(ctx)
		if err != nil {
			return fmt.Errorf("failed to open %v: %v", obj, err)
		}
		err = fn(rd)
		rd.Close()
		if err != nil {
			return fmt.Errorf("error processing %v: %v", obj, err)
		}
	}
	return nil
}

// GetMutations returns a channel of mutations or related events.
// The channel will never be closed.
// All sends on the returned channel should select on the provided context.
func (gl *GCSLog) GetMutations(ctx context.Context) <-chan maintner.MutationStreamEvent {
	ch := make(chan maintner.MutationStreamEvent, 50) // buffered: overlap gunzip/unmarshal with loading
	go func() {
		err := gl.foreachSegmentReader(ctx, func(r io.Reader) error {
			return reclog.ForeachRecord(r, 0, func(off int64, hdr, rec []byte) error {
				m := new(maintpb.Mutation)
				if err := proto.Unmarshal(rec, m); err != nil {
					return err
				}
				select {
				case ch <- maintner.MutationStreamEvent{Mutation: m}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		})
		final := maintner.MutationStreamEvent{Err: err}
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

func try(tries int, firstDelay time.Duration, fn func() error) error {
	var err error
	delay := firstDelay
	for i := 0; i < tries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
}

// CopyFrom is only used for the one-time migrate from disk-to-GCS code path.
func (gl *GCSLog) CopyFrom(src maintner.MutationSource) error {
	gl.curNum = 0
	ctx := context.Background()
	for e := range src.GetMutations(ctx) {
		if e.Err != nil {
			log.Printf("Corpus.Initialize: %v", e.Err)
			return e.Err
		}
		if e.End {
			log.Printf("reached end. flushing.")
			err := gl.flush(ctx)
			log.Printf("final flush = %v", err)
			return nil
		}
		if err := gl.Log(e.Mutation); err != nil {
			return err
		}
	}
	panic("unexpected channel close")
}

// RegisterHandlers adds handlers for the default paths (/logs and /logs/).
func (gl *GCSLog) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/logs", gl.serveJSONLogsIndex)
	mux.HandleFunc("/logs/", gl.serveLogFile)
}
