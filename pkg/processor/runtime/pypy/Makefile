run: go2pypy
	GODEBUG=cgocheck=0 ./pypy

go2pypy: *.go interface.py
	go build .

