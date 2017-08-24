# v3io

V3IO SDK for Go language 


Usage example:
```go
// Create V3IO Data Container context 
v3 := v3io.V3iow{"http://192.168.152.27:8081/2",&http.Transport{},true}

// List all objects in directory/prefix "streams" 
resp,err := v3.ListBucket("streams")
  
// Get a stream of records 
v,_ := v3.SeekShard("streams/mytopic","0","LATEST",0)
fmt.Printf("Seek Location: %d\n", v)

resp,err = v3.GetRecords("streams/mytopic","0",v,10,0)
fmt.Printf("Resp: %s\n", resp.Records[0].Data)

// Get item obj7 from table "table" with specific attributes 
resp, err = v3.GetItem("table", "obj7", "__size,__mtime_secs,Name,Age")
```

