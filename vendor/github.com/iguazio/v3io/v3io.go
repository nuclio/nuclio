package v3io

import (
	"fmt"
	"encoding/json"
	"reflect"
	"net/http"
	"io/ioutil"
	"bytes"
	"encoding/xml"
	"strings"
	"os"
	"runtime"
	"strconv"
	"time"
	"encoding/base64"
)

type V3iow struct {
	Url string
	//Container string
	Tr http.RoundTripper
	DebugState bool
}

type V3ioConf struct {
	Version string
	Root_path, Fuse_Path string
	Debug bool
	Clusters []struct{ Name, Data_url, Api_url, Web_url string}
}

type ListAllResp struct {
	XMLName     xml.Name `xml:"ListAllMyBucketsResult"`
	Owner	    interface{}   `xml:"Owner"`
	Buckets     BckList `xml:"Buckets"`
}

type BckList struct {
	XMLName    xml.Name `xml:"Buckets"`
	Bucket []BckListItm   `xml:"Bucket"`
}

type BckListItm struct {
	XMLName    xml.Name `xml:"Bucket"`
	Name	   string   `xml:"Name"`
	CreationDate	   string   `xml:"CreationDate"`
	Id   int   `xml:"Id"`
}



type ListBucketResp struct {
	XMLName     xml.Name `xml:"ListBucketResult"`
	Name	    string   `xml:"Name"`
	NextMarker  string   `xml:"NextMarker"`
	MaxKeys     string   `xml:"MaxKeys"`
	Contents    []BContent `xml:"Contents"`
	CommonPrefixes  []BPrefixes `xml:"CommonPrefixes"`
}

type BContent struct {
	XMLName    xml.Name `xml:"Contents"`
	Key string   `xml:"Key"`
	Size   int   `xml:"Size"`
	LastSequenceId   int   `xml:"LastSequenceId"`
	ETag   string   `xml:"ETag"`
	LastModified string `xml:"LastModified"`
}

type BPrefixes struct {
	XMLName    xml.Name `xml:"CommonPrefixes"`
	Prefix string   `xml:"Prefix"`
}

type GetItemResp struct {
	Item ItemRespStruct
}

type ItemRespStruct map[string]interface{}

type tmpst map[string]string


func (it *ItemRespStruct) UnmarshalJSON(bs []byte) error {
	item := make(map[string]tmpst)
	*it = make(map[string]interface{})

	if err := json.Unmarshal(bs, &item); err != nil {
		return err
	}
	for k,v := range item {
		for tp,v2 := range v {
			var err error
			err = nil
			switch tp {
			case "S":
				(*it)[k] = v2
			case "N":
				(*it)[k] , err = strconv.Atoi(v2)
			case "B":
				(*it)[k] , err = base64.StdEncoding.DecodeString(v2)
			}
			if err != nil { return err }
		}
	}

	return nil
}

type GetItemsResp struct {
	LastItemIncluded string
	NextMarker string
	NumItems int
	Items []ItemRespStruct
}


type StreamRecord struct {
	HasShard bool
	ShardId int
	Data string
	ClientEventTime time.Time
}

type GetRecordsResp struct {
	NextLocation int
	LagInBytes, LagInMsec, LagInRecord int
	RecordsNum int
	Records []GetRecordsRec
}

type GetRecordsRec struct {
	SequenceNumber, ArrivalTimestamp int
	Data []byte
	ClientEventTimeSec int
	ClientEventTimeNSec int
}


// Trace function
func Fullerr(format interface{}, v ...interface{}) error {
	f := fmt.Sprintf("%v",format)

	pfx := ""
	fpcs, _, no, ok := runtime.Caller(1)
	if ok {
		fun := runtime.FuncForPC(fpcs)
		pfx = fmt.Sprintf("Error! in %s #%d ", fun.Name(), no)
		//fmt.Println(fun.Name())
	}
	return fmt.Errorf(pfx+f, v...)
}

func (v3 V3iow) debug(format interface{}, vars ...interface{})  {
	if v3.DebugState {fmt.Printf(fmt.Sprintf("%v",format),vars...) }
}



func (v3 V3iow) ListAll(fullpath string) (ListAllResp,error) {
	var htmlData []byte
	la := ListAllResp{}
	res, err := http.Get(fullpath)
	if err != nil {return la, Fullerr(err)}
	defer res.Body.Close()
	v3.debug("Stat %s\n", res.Status)
	if res.StatusCode != 200 {return la, Fullerr(res.Status)}
	htmlData, err = ioutil.ReadAll(res.Body)
	if err != nil {return la, Fullerr(err)}
	v3.debug("Resp: %s\n",htmlData)
	err = xml.Unmarshal(htmlData, &la)
	if err != nil {	return la, Fullerr("failed to Unmarshal %s (%v)",fullpath,err) }
	return la,nil
}

func (v3 V3iow) ListBucket(path string) (ListBucketResp,error) {
	var htmlData []byte
	lb := ListBucketResp{}
	if v3.Url == "" {
		file, err := os.Open("tst.xml") // For read access.
		if err != nil {panic(err)}
		defer file.Close()
		htmlData, err = ioutil.ReadAll(file)
		if err != nil {panic(err)}
	} else {
		//client := &http.Client{Transport: v3.Tr}
		pathstr := ""
		if path != "" { pathstr = "?prefix="+path}
		fullpath := strings.Join([]string{v3.Url,pathstr},"/")
		v3.debug("Path: %s\n",fullpath)
		res, err := http.Get(fullpath)
		if err != nil {return lb, Fullerr(err)}
		defer res.Body.Close()
		v3.debug("Stat %s\n", res.Status)
		if res.StatusCode != 200 {return lb, Fullerr(res.Status)}
		htmlData, err = ioutil.ReadAll(res.Body)
		if err != nil {return lb, Fullerr(err)}
		v3.debug("Resp: %s\n",htmlData)
	}
	err := xml.Unmarshal(htmlData, &lb)
	if err != nil {	return lb, Fullerr("failed to Unmarshal %s (%v)",path,err) }
	return lb,nil
}


func (v3 V3iow) Get(path string) ([]byte,error) {
	var Data []byte
	//client := &http.Client{Transport: v3.Tr}
	fullpath := strings.Join([]string{v3.Url,path},"/")
	v3.debug("Path: %s\n",fullpath)
	res, err := http.Get(fullpath)
	if err != nil {return Data, Fullerr(err)}
	defer res.Body.Close()
	v3.debug("Stat %s\n", res.Status)
	if res.StatusCode != 200 {return Data, Fullerr(res.Status)}
	Data, err = ioutil.ReadAll(res.Body)
	if err != nil {return Data, Fullerr(err)}
	v3.debug("Resp: %s\n",Data)
	return Data,nil
}

func (v3 V3iow) Put(path string, body []byte) ([]byte,error) {
	client := &http.Client{Transport: v3.Tr}
	fullpath := strings.Join([]string{v3.Url,path},"/")
	req, err := http.NewRequest("PUT", fullpath, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("V3iow:Put - failed New req %s (%v)", fullpath,err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("V3iow:Put - failed to Do req %s (%v)", fullpath,err)
	}
	defer res.Body.Close()
	v3.debug("Stat: %s\n", res.Status)

	//fmt.Printf("Stat %s\n", res.Status)
	htmlData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("V3iow:Put - failed to read body %s (%v)", fullpath,err)
	}
	return htmlData, nil
}


func (v3 V3iow) UpdateItem(path string, list map[string]interface{}) ([]byte,error) {
	return v3.UpdateItemEx(path,list,"UpdateItem")
}

func (v3 V3iow) PutItem(path string, list map[string]interface{}) ([]byte,error) {
	return v3.UpdateItemEx(path,list,"PutItem")
}

func (v3 V3iow) UpdateItemEx(path string, list map[string]interface{}, cmd string) ([]byte,error) {
	atrs := make(map[string]map[string]string)
	for key, val := range list  {
		atrs[key] = make(map[string]string)
		switch v := val.(type) {
		default:
			fmt.Printf("unexpected type %s : %T", reflect.TypeOf(val), v)
		case int:
			atrs[key]["N"] = fmt.Sprintf("%d",val)
		case float64: // this is a tmp bypass to the fact Go maps Json numbers to float64
			atrs[key]["N"] = fmt.Sprintf("%d",int(val.(float64)))
		case string:
			atrs[key]["S"] = val.(string)
		case []byte:
			atrs[key]["B"] = base64.StdEncoding.EncodeToString(val.([]byte))
		}
	}
	// create PutItem Body
	b := make(map[string]interface{})
	//keys := make(map[string]map[string]string)
	//keys["ID"] = make(map[string]string)
	//keys["ID"]["S"] = key
	//b["Key"] = keys

	b["Item"] = atrs
	if cmd=="UpdateItem" { b["UpdateMode"]="CreateOrReplaceAttributes"}

	body, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("V3iow:Update/PutItem - failed to Marshal %v (%v)", b,err)
	}
	fullpath := v3.Path2url(path,"")
	v3.debug("%s %s to Path: %s\n",cmd,body,fullpath)
	resp,err := v3.PostRequest(fullpath,cmd,body)
	v3.debug("Resp: %s\n",resp)
	return resp,err
}

func (v3 V3iow) GetItem(path,attrs string) (GetItemResp,error) {
	// create GetItem Body
	var resp GetItemResp
	b := make(map[string]string)
	b["AttributesToGet"] = attrs
	body, err := json.Marshal(b)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetItem - failed to Marshal %v (%v)", b,err)
	}

	fullpath := v3.Path2url(path,"")
	v3.debug("GetItem %s from Path: %s",body,fullpath)
	body,err = v3.PostRequest(fullpath,"GetItem",body)
	if err != nil {
		return resp, err
	}
	v3.debug("Body: %s\n",body)
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetItem - failed to Unmarshal resp %v (%v)", body,err)
	}

	return resp,nil
}

func (v3 V3iow) GetItems(path,attrs,filter,marker string, limit,seg,totalseg int) (GetItemsResp,error) {
	// create GetItem Body
	var resp GetItemsResp
	b := make(map[string]interface{})
	b["AttributesToGet"] = attrs
	if filter != "" {b["FilterExpression"]=filter}
	if marker != "" {b["Marker"]=marker}
	if limit != 0 {b["Limit"]=limit}
	if totalseg != 0 {
		b["TotalSegment"]=totalseg
		b["Segment"]=seg
	}
	body, err := json.Marshal(b)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetItems - failed to Marshal %v (%v)", b,err)
	}

	fullpath := v3.Path2url(path,"")
	v3.debug("GetItems %s from Path: %s",body,fullpath)
	body,err = v3.PostRequest(fullpath,"GetItems",body)
	if err != nil {
		return resp, err
	}
	v3.debug("Body: %s\n",body)
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetItems - failed to Unmarshal resp %v (%v)", body,err)
	}

	return resp,nil
}

func (v3 V3iow) PutRecordsEx(path string,records []StreamRecord) ([]byte,error) {
	// create PutRecords Body
	reqstr := []string{}
	for _, rec := range records  {
		r := fmt.Sprintf("%q : %q", "Data",base64.StdEncoding.EncodeToString([]byte(rec.Data)))
		if rec.HasShard { r = fmt.Sprintf("%q : %d, ", "ShardId",rec.ShardId) + r}
		reqstr = append(reqstr,"   { " + r + " }\n")
	}
	body := []byte(fmt.Sprintf("{\n %q : [\n %s  ]\n}", "Records",strings.Join(reqstr,",")))

	fullpath := v3.Path2url(path,"")
	v3.debug("PutRecords \n%s\n to Path: %s\n",body,fullpath)
	resp,err := v3.PostRequest(fullpath,"PutRecords",body)
	v3.debug("Resp: %s\n",resp)
	return resp,err
}

func (v3 V3iow) PutRecords(path string,records []string) ([]byte,error) {
	var recs []StreamRecord
	for _,v := range records {
		r := StreamRecord{ Data:v}
		recs = append(recs, r)
	}
	return v3.PutRecordsEx(path ,recs)
}


func (v3 V3iow) GetRecords(path string,offset,maxrec,startseq int ) (GetRecordsResp,error) {
	// create GetItem Body
	var resp GetRecordsResp
	b := make(map[string]int)
	b["Location"] = offset
	//b["StartingSequenceNumber"] = startseq
	b["MaxRecords"] = maxrec
	body, err := json.Marshal(b)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetRecords - failed to Marshal %v (%v)", b,err)
	}

	fullpath := v3.Path2url(path,"")
	v3.debug("GetRecords %s from Path: %s",body,fullpath)
	body,err = v3.PostRequest(fullpath,"GetRecords",body)
	if err != nil {
		return resp, err
	}
	v3.debug("Body: %s\n",body)
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return resp, fmt.Errorf("V3iow:GetRecords - failed to Unmarshal resp %v (%v)", body,err)
	}
	return resp,nil
}

func (v3 V3iow) SeekShard(path string, seek string, from int) (int,error) {
	// Types:  SEQUENCE (+ starting), TIME (+Time stamp), LATEST, EARLIEST
	r := fmt.Sprintf("%q : %q", "Type",seek)
	if seek == "SEQUENCE"  { r += fmt.Sprintf(",%q : %d", "StartingSequenceNumber",from) }
	if seek == "TIME"  { r += fmt.Sprintf(",%q : %d", "TimeStamp",from) }
	body := []byte("{" + r + "}")

	fullpath := v3.Path2url(path,"")
	v3.debug("SeekShard %s from Path: %s",body,fullpath)
	body,err := v3.PostRequest(fullpath,"SeekShard",body)
	if err != nil {
		return 0, err
	}
	v3.debug("Body: %s\n",body)

	type respstruct struct { Location int }
	var resp respstruct
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, fmt.Errorf("V3iow:SeekShard - failed to Unmarshal resp %v (%v)", body,err)
	}
	return resp.Location , nil
}

func (v3 V3iow) CreateStream(path string, count,mbsize int) ([]byte,error) {
	// Types:  SEQUENCE (+ starting), TIME (+Time stamp), LATEST, EARLIEST
	r := fmt.Sprintf("{ %q:%d, %q:%d }", "ShardCount",count,"ShardRetentionPeriodSizeMB",mbsize)
	body := []byte(r)

	fullpath := v3.Path2url(path,"")
	v3.debug("CreateStream %s from Path: %s",body,fullpath)
	resp,err := v3.PostRequest(fullpath,"CreateStream",body)
	v3.debug("Resp: %s\n",resp)
	return resp,err
}


func (v3 V3iow) PostRequest(fullpath,fname string,body []byte) ([]byte,error) {
	//fmt.Printf("%s - %s \n%s\n",fullpath,fname,body)
	if v3.Url == "" {fullpath="https://echo.getpostman.com/post"}
	client := &http.Client{Transport: v3.Tr}
	req, _ := http.NewRequest("POST", fullpath, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-v3io-function", fname)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("V3iow:PostRequest - failed to Do req %s (%v)", fullpath,err)
	}
	defer res.Body.Close()
	v3.debug("Stat: %s\n", res.Status)

	//fmt.Printf("Stat %s\n", res.Status)
	htmlData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("V3iow:PostRequest - failed to read body %s (%v)", fullpath,err)
	}
	return htmlData, nil
}

func (v3 V3iow) Path2url(path,key string) string {
	tmp:= v3.Url
	if path !="" {tmp = tmp + "/" + path}
	if key  !="" {tmp = tmp + "/" + key}
	return tmp
}

func V3test() {
	//v3 := V3iow{"", "4242865025", &http.Transport{}}
	v3 := V3iow{"http://192.168.152.27:8081/3964515741",&http.Transport{},true}
	res,err := v3.ListBucket("table")
	if err != nil {panic(err)}
	fmt.Printf("Resp: %v\n", res.Contents)

	tst := map[string]interface{}{"Age":55, "Name":"Joe"}
	body, err := v3.PutItem("table/obj7", tst)
	if err != nil {panic(err)}
	fmt.Printf("Resp: %s\n", body)
	//fmt.Printf("xx %s\n",body)

	resp, err := v3.GetItem("table/obj7", "__size,__mtime_secs,Name,Age")
	if err != nil {panic(err)}
	fmt.Printf("Resp: %+v\n", resp)

}