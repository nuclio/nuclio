# RPC
[![GoDoc](https://godoc.org/gopkg.in/jcmturner/rpc.v1?status.svg)](https://godoc.org/gopkg.in/jcmturner/rpc.v1) [![Go Report Card](https://goreportcard.com/badge/gopkg.in/jcmturner/rpc.v1)](https://goreportcard.com/report/gopkg.in/jcmturner/rpc.v1) [![Build Status](https://travis-ci.org/jcmturner/rpc.svg?branch=master)](https://travis-ci.org/jcmturner/rpc)


This project relates to [CDE 1.1: Remote Procedure Call](http://pubs.opengroup.org/onlinepubs/9629399/)

It is a partial implementation that mainly focuses on unmarshaling NDR encoded byte streams into Go structures.

## Help Wanted
**Reference test vectors needed**: It has been difficult to implement due to a lack of reference test byte streams in the 
standards documentation. Test driven development has been extremely challenging without these.
If you are aware of and reference test vector sources for NDR encoding please let me know by raising an issue with the details. Thanks!

## References
* [Open Group RPC Publication](http://pubs.opengroup.org/onlinepubs/9629399/)
* [Microsoft RPC Documentation](https://docs.microsoft.com/en-us/windows/desktop/Rpc/rpc-start-page)

## NDR Decode Capability Checklist
- [x] Format label
- [x] Boolean
- [x] Character
- [x] Unsigned small integer
- [x] Unsigned short integer
- [x] Unsigned long integer
- [x] Unsigned hyper integer
- [x] Signed small integer
- [x] Signed short integer
- [x] Signed long integer
- [x] Signed hyper integer
- [x] Single float
- [x] Double float
- [x] Uni-dimensional fixed array
- [x] Multi-dimensional fixed array
- [x] Uni-dimensional conformant array
- [x] Multi-dimensional conformant array
- [x] Uni-dimensional conformant varying array
- [x] Multi-dimensional conformant varying array
- [x] Varying string
- [x] Conformant varying string
- [x] Array of strings
- [x] Union
- [x] Pipe

## Structs from IDL
[Interface Definition Language (IDL)](http://pubs.opengroup.org/onlinepubs/9629399/chap4.htm)

### Is a field a pointer?

### Is an array conformant and/or varying?
An array is conformant if the IDL definition includes one of the following attributes:
* min_is
* max_is
* size_is

An array is varying if the IDL definition includes one of the following attributes: 
* last_is
* first_is 
* length_is

#### Examples:
SubAuthority[] is conformant in the example below:
```
 typedef struct _RPC_SID {
   unsigned char Revision;
   unsigned char SubAuthorityCount;
   RPC_SID_IDENTIFIER_AUTHORITY IdentifierAuthority;
   [size_is(SubAuthorityCount)] unsigned long SubAuthority[];
 } RPC_SID,
  *PRPC_SID,
  *PSID;
```

Buffer is a pointer to a conformant varying array in the example below:
```
 typedef struct _RPC_UNICODE_STRING {
   unsigned short Length;
   unsigned short MaximumLength;
   [size_is(MaximumLength/2), length_is(Length/2)] 
     WCHAR* Buffer;
 } RPC_UNICODE_STRING,
  *PRPC_UNICODE_STRING;
```

### Is a union encapsulated?