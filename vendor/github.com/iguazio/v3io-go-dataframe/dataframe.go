package dataframe

import (
	"bytes"
	"fmt"
	"github.com/v3io/v3io-go-http"
	"github.com/nuclio/nuclio-sdk"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"io"
	"strconv"
	"strings"
)

var Writer writersList

func NewDataContext(logger nuclio.Logger) *DataContext {
	dc := DataContext{}
	dc.Write = newWritersList(&dc)
	dc.Read = newReaderList(&dc)
	dc.respChan = make(chan *v3io.Response, 100)
	dc.reqMap = map[uint64]interface{}{}
	dc.logger = logger
	return &dc
}

type DataContext struct {
	logger   nuclio.Logger
	Write    *writersList
	Read     *readersList
	respChan chan *v3io.Response
	reqMap   map[uint64]interface{}
}

func NewAsyncGroup() *asyncGroup {
	ag := asyncGroup{tasks: []interface{}{}}
	return &ag

}

type asyncGroup struct {
	respChan chan *v3io.Response
	tasks    []interface{}
	err      error
}

func (w *asyncGroup) Wait() error {
	return nil

}

func (dc *DataContext) WaitForCompletions(writers ...*writer) error {

	var err error
	pending := len(dc.reqMap)

	for numResponses := 0; numResponses < pending; numResponses++ {
		response := <-dc.respChan

		wr := dc.reqMap[response.ID].(*writer)
		if response.Error != nil {
			err = response.Error
			wr.err = response.Error
			dc.logger.ErrorWith("failed WaitForCompletion", "err", wr.err, "path", wr.path+"/"+wr.key)
		} else {
			wr.output = response.Output
		}
		delete(dc.reqMap, response.ID)
	}
	return err
}

func (dc *DataContext) SaveAll(writers ...*writer) {

	respChan := make(chan *v3io.Response, len(writers))
	wmap := map[uint64]*writer{}
	submitted := 0

	for _, wr := range writers {
		resp, err := wr.submit(respChan)
		if err == nil {
			wmap[resp.ID] = wr
			submitted++
		}
	}

	for numResponses := 0; numResponses < submitted; numResponses++ {
		response := <-respChan

		wr := wmap[response.ID]
		if response.Error != nil {
			wr.err = response.Error
		} else {
			wr.output = response.Output
		}
	}
}

func RenderTable(output io.Writer, header []string, records [][]string) {
	tableWriter := tablewriter.NewWriter(output)
	tableWriter.SetHeader(header)
	tableWriter.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})
	tableWriter.SetCenterSeparator("|")
	tableWriter.SetHeaderLine(false)
	tableWriter.AppendBulk(records)
	tableWriter.Render()
}

// --------------------  READER

type readersList struct {
	dc *DataContext
	//FromTable  func(source *v3io.Container, table string) *frame
}

func newReaderList(dc *DataContext) *readersList {
	rl := readersList{dc: dc}
	return &rl
}

func (rl *readersList) FromTable(source interface{}, path string) *dataSource {
	return newDataSource(rl.dc, source, path)
}

type dataFrame struct {
	dc       *DataContext
	keys     map[string]int
	data     []tableRow
	cursor   int
	loading  bool
	err      error
	respChan chan *v3io.Response
}

type tableRow map[string]interface{}

func (tr tableRow) Col(name string) tableField {
	field, ok := tr[name]
	if !ok {
		field = ""
	}
	return tableField{field: field}
}

func (tr tableRow) Scan(fields string, pointers ...interface{}) error {
	list := strings.Split(fields, ",")
	if len(list) != len(pointers) {
		return fmt.Errorf("number of fields (comma seperated) must match number of pointers)")
	}
	for idx, name := range list {
		field, ok := tr[name]
		if !ok {
			field = ""
		}
		p := pointers[idx]
		switch p.(type) {
		case *string:
			*p.(*string) = asString(field)
		case *int:
			*p.(*int) = asInt(field)

		}

	}
	return nil
}

type tableField struct {
	field interface{}
}

type fieldCast interface {
	AsInt() int
	AsString() string
}

func (f tableField) AsInt() int {
	val, ok := f.field.(int)
	if ok {
		return val
	}
	return 0
}

func asInt(num interface{}) int {
	val, ok := num.(int)
	if ok {
		return val
	}
	return 0
}

func asString(val interface{}) string {
	switch val.(type) {
	case string:
		return val.(string)
	case int:
		return strconv.Itoa(val.(int))
	}
	return ""
}

func (f tableField) AsStr() string {
	switch f.field.(type) {
	case string:
		return f.field.(string)
	case int:
		return strconv.Itoa(f.field.(int))
	}
	return ""
}

type tableColumn struct {
	ds      *dataSource
	colname string
}

func (c *tableColumn) colStrings() []string {

	col := []string{}
	for _, row := range c.ds.data {
		field, ok := (*row)[c.colname]
		if ok {
			col = append(col, asString(field))
		}
	}
	return col
}

func newDataSource(dc *DataContext, source interface{}, path string) *dataSource {
	ds := dataSource{dc: dc, source: source, path: path}
	return &ds
}

type dataSource struct {
	dc         *DataContext
	source     interface{}
	path       string
	keys       []interface{}
	attributes []string
	filter     string
	data       []*tableRow
	cursor     int
	loading    bool
	respChan   chan *v3io.Response
	respMap    map[uint64]string
	itemsCurs  *ItemsCursor
	err        error
}

func (ds *dataSource) addRow(row *map[string]interface{}) *tableRow {
	newrow := tableRow{}
	for k, v := range *row {
		newrow[k] = v
	}
	ds.data = append(ds.data, &newrow)
	return &newrow
}

func (ds *dataSource) Keys(keys ...interface{}) *dataSource {
	ds.keys = keys
	return ds
}

func (ds *dataSource) getAttr() *[]string {
	if len(ds.attributes) == 0 {
		any := []string{"*"}
		return &any
	}
	return &ds.attributes
}

func (ds *dataSource) getKeys() []string {
	if len(ds.keys) == 0 {
		return []string{}
	}
	switch ds.keys[0].(type) {
	case *tableColumn:
		return ds.keys[0].(*tableColumn).colStrings()
	}
	keys := []string{}
	for _, key := range ds.keys {
		switch key.(type) {
		case string:
			keys = append(keys, key.(string))
		}

	}
	return keys
}

func (ds *dataSource) GetRow() *tableRow {
	// TODO for get items
	keys := ds.getKeys()
	if len(keys) == 0 {
		ds.err = ds.loadItems()
		if ds.err != nil {
			ds.dc.logger.ErrorWith("failed frames LoadAsync items", "err", ds.err, "path", ds.path)
			return nil
		}
		row := ds.Next()
		return &row
	}
	resp, err := ds.source.(*v3io.Container).Sync.GetItem(&v3io.GetItemInput{
		Path: ds.path + "/" + keys[0], AttributeNames: *ds.getAttr()})
	if err != nil {
		ds.err = err
		ds.dc.logger.ErrorWith("failed frame getrow", "err", err, "path", ds.path)
		return nil
	}
	var row map[string]interface{}
	row = resp.Output.(*v3io.GetItemOutput).Item
	row["__name"] = keys[0]
	return ds.addRow(&row)
}

func (ds *dataSource) LoadAsync() *dataSource {

	// TODO: block more Loads, do pre-fetch, add GetItems

	if len(ds.keys) == 0 {
		// TODO: need to use async calls, maybe a go routine to pre-fetch
		ds.err = ds.loadItems()
		if ds.err != nil {
			ds.dc.logger.ErrorWith("failed frames LoadAsync items", "err", ds.err, "path", ds.path)
		}
		return ds
	}

	keys := ds.getKeys()
	if len(keys) == 0 {
		return ds
	}

	ds.respChan = make(chan *v3io.Response, len(keys))
	ds.respMap = map[uint64]string{}

	for _, key := range keys {
		resp, err := ds.source.(*v3io.Container).GetItem(&v3io.GetItemInput{
			Path: ds.path + "/" + key, AttributeNames: *ds.getAttr()}, ds.respChan)
		if err != nil {
			ds.err = err
			ds.dc.logger.ErrorWith("failed frames LoadAsync by key", "err", err, "path", ds.path+"/"+key)
		} else {
			ds.respMap[resp.ID] = key
		}
	}

	return ds
}

func (ds *dataSource) getDataResp(num int) error {
	submitted := len(ds.respMap)
	var err error
	if num > submitted {
		num = submitted
	}
	for numResponses := 0; numResponses < num; numResponses++ {
		response := <-ds.respChan

		key := ds.respMap[response.ID]
		if response.Error != nil {
			ds.err = response.Error
			ds.dc.logger.ErrorWith("failed frames get resp", "err", ds.err, "path", ds.path+"/"+key)
			err = ds.err
		} else {
			var row map[string]interface{}
			row = response.Output.(*v3io.GetItemOutput).Item
			row["__name"] = key
			ds.addRow(&row)
		}
		delete(ds.respMap, response.ID)
	}

	return err
}

func (ds *dataSource) Col(colname string) *tableColumn {
	return &tableColumn{ds: ds, colname: colname}
}

func (ds *dataSource) Next() tableRow {

	if len(ds.keys) == 0 {
		// TODO: do a better imp
		item, err := ds.itemsCurs.Next()
		if err != nil {
			ds.err = err
			return nil
		}
		var row map[string]interface{}
		row = *item
		ds.addRow(&row)
		ds.cursor += 1
		return *ds.data[ds.cursor-1]
	}

	if len(ds.data) <= ds.cursor || len(ds.respMap) > 0 {
		ds.getDataResp(1)
		// TODO: pre-fetch
	}
	if ds.data == nil || len(ds.data) <= ds.cursor {
		return nil
	}
	ds.cursor += 1
	return *ds.data[ds.cursor-1]
}

func (ds *dataSource) Rows() []*tableRow {

	if len(ds.keys) == 0 {
		// TODO: do a better imp
		if ds.err != nil {
			return []*tableRow{}
		}
		items, err := ds.itemsCurs.All()
		if err != nil {
			ds.err = err
			return nil
		}
		for _, item := range items {
			var row map[string]interface{}
			row = *item
			ds.addRow(&row)
		}
		return ds.data
	}

	if len(ds.respMap) > 0 {
		ds.getDataResp(len(ds.respMap))
	}
	if ds.data == nil || len(ds.data) == 0 {
		return nil
	}
	return ds.data
}

func (ds *dataSource) colMap() (int, map[string][]string) {
	rows := ds.Rows()
	colMap := map[string][]string{}
	colMap["__name"] = make([]string, len(rows))
	for i, row := range rows {
		for k, v := range *row {
			if _, ok := colMap[k]; !ok {
				colMap[k] = make([]string, len(rows))
			}
			colMap[k][i] = asString(v)
		}
	}
	return len(rows), colMap
}

func (ds *dataSource) ToTable() []byte {
	size, colMap := ds.colMap()
	if size == 0 {
		return []byte{}
	}
	headers := []string{}
	data := [][]string{}

	for k, _ := range colMap {
		headers = append(headers, k)
	}

	for i := 0; i < size; i++ {
		row := []string{}
		for _, h := range headers {
			row = append(row, colMap[h][i])
		}
		data = append(data, row)
	}

	buf := new(bytes.Buffer)
	RenderTable(buf, headers, data)
	return buf.Bytes()
}

func (ds *dataSource) Error() error {
	return ds.err
}

func (ds *dataSource) Select(attrs ...string) *dataSource {
	ds.attributes = attrs
	return ds
}

func (ds *dataSource) Where(filter string) *dataSource {
	ds.filter = filter
	return ds
}

func (ds *dataSource) loadItems() error {

	input := v3io.GetItemsInput{Path: ds.path, AttributeNames: *ds.getAttr(), Filter: ds.filter}

	response, err := ds.source.(*v3io.Container).Sync.GetItems(&input)
	//time.Sleep(time.Second)
	if err != nil {
		//ds.dc.logger.ErrorWith("Failed GetItems with:","error", err)
		return err
	}
	ds.itemsCurs = newItemsCursor(ds.source.(*v3io.Container), &input, response)
	return nil
}

//-------

type ItemsCursor struct {
	nextMarker     string
	moreItemsExist bool
	itemIndex      int
	items          []v3io.Item
	response       *v3io.Response
	input          *v3io.GetItemsInput
	container      *v3io.Container
}

func newItemsCursor(container *v3io.Container, input *v3io.GetItemsInput, response *v3io.Response) *ItemsCursor {
	newItemsCursor := &ItemsCursor{
		container: container,
		input:     input,
	}

	newItemsCursor.setResponse(response)

	return newItemsCursor
}

// release a cursor and its underlying resources
func (ic *ItemsCursor) Release() {
	ic.response.Release()
}

// get the next matching item. this may potentially block as this lazy loads items from the collection
func (ic *ItemsCursor) Next() (*v3io.Item, error) {

	// are there any more items left in the previous response we received?
	if ic.itemIndex < len(ic.items) {
		item := &ic.items[ic.itemIndex]

		// next time we'll give next item
		ic.itemIndex++

		return item, nil
	}

	// are there any more items up stream?
	if !ic.moreItemsExist {
		return nil, nil
	}

	// get the previous request input and modify it with the marker
	ic.input.Marker = ic.nextMarker

	// invoke get items
	newResponse, err := ic.container.Sync.GetItems(ic.input)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to request next items")
	}

	// release the previous response
	ic.response.Release()

	// set the new response - read all the sub information from it
	ic.setResponse(newResponse)

	// and recurse into next now that we repopulated response
	return ic.Next()
}

// gets all items
func (ic *ItemsCursor) All() ([]*v3io.Item, error) {
	items := []*v3io.Item{}

	for {
		item, err := ic.Next()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get next item")
		}

		if item == nil {
			break
		}

		items = append(items, item)
	}

	return items, nil
}

func (ic *ItemsCursor) setResponse(response *v3io.Response) {
	ic.response = response

	getItemsOutput := response.Output.(*v3io.GetItemsOutput)

	ic.moreItemsExist = !getItemsOutput.Last
	ic.nextMarker = getItemsOutput.NextMarker
	ic.items = getItemsOutput.Items
	ic.itemIndex = 0
}

// -------- WRITER

type writersList struct {
	dc     *DataContext
	logger nuclio.Logger
}

func newWritersList(dc *DataContext) *writersList {
	wl := writersList{dc: dc}
	return &wl
}

func (wl *writersList) ToTable(source interface{}, path string) *writer {
	return &writer{source: source, path: path, dc: wl.dc}
}

func (wl *writersList) ToStream(source interface{}, path string) *writer {
	return &writer{source: source, path: path, dc: wl.dc}
}

type writer struct {
	dc         *DataContext
	source     interface{}
	path       string
	key        string
	shard      string
	writerType writerType
	data       interface{}
	output     interface{}
	err        error
}

type writerType int

const (
	writerTypeUnset = iota
	writerTypePutRecords
	writerTypeUpdateAttr
	writerTypeUpdateExpr
	writerTypePutObj
)

func (w *writer) Error() error {
	return w.err
}

func (w *writer) Keys(key string) *writer {
	w.key = key
	return w
}

func (w *writer) Records(records ...[]byte) *writer {
	w.writerType = writerTypePutRecords
	w.data = records
	return w
}

func (w *writer) Expression(expression string) *writer {
	w.writerType = writerTypeUpdateExpr
	w.data = expression
	return w
}

func (w *writer) Condition(condition string) *writer {
	return w
}

func (w *writer) submit(respChan chan *v3io.Response) (*v3io.Request, error) {
	var err error
	var resp *v3io.Request

	switch w.writerType {
	case writerTypePutRecords:
		records := []*v3io.StreamRecord{}
		for _, r := range w.data.([][]byte) {
			rec := v3io.StreamRecord{Data: r}
			records = append(records, &rec)
		}
		resp, err = w.source.(*v3io.Container).PutRecords(&v3io.PutRecordsInput{
			Path: w.path + "/", Records: records}, respChan)

	case writerTypeUpdateExpr:
		expression := w.data.(string)
		resp, err = w.source.(*v3io.Container).UpdateItem(&v3io.UpdateItemInput{
			Path: w.path + "/" + w.key, Expression: &expression}, respChan)

	case writerTypeUpdateAttr:
		resp, err = w.source.(*v3io.Container).UpdateItem(&v3io.UpdateItemInput{
			Path: w.path + "/" + w.key, Attributes: w.data.(map[string]interface{})}, respChan)

	}

	w.err = err
	return resp, err

}

func (w *writer) Save() *writer {
	respChan := make(chan *v3io.Response, 1)
	_, err := w.submit(respChan)
	if err != nil {
		w.dc.logger.ErrorWith("failed save submit", "err", err, "path", w.path)
		return w
	}
	response := <-respChan
	if response.Error != nil {
		w.err = response.Error
		w.dc.logger.ErrorWith("failed save response", "err", err, "path", w.path)
	} else {
		w.output = response.Output
	}
	return w
}

func (w *writer) SaveAsync() *writer {
	resp, err := w.submit(w.dc.respChan)
	if err == nil {
		w.dc.reqMap[resp.ID] = w
	} else {
		w.dc.logger.ErrorWith("failed async save submit", "err", err, "path", w.path)
	}
	return w
}
