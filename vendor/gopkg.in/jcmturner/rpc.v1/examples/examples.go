// Package examples provides example decoding of NDR byte streams
package examples

import "gopkg.in/jcmturner/rpc.v1/mstypes"

type KerbValidationInfo struct {
	LogOnTime              mstypes.FileTime
	LogOffTime             mstypes.FileTime
	KickOffTime            mstypes.FileTime
	PasswordLastSet        mstypes.FileTime
	PasswordCanChange      mstypes.FileTime
	PasswordMustChange     mstypes.FileTime
	EffectiveName          mstypes.RPCUnicodeString
	FullName               mstypes.RPCUnicodeString
	LogonScript            mstypes.RPCUnicodeString
	ProfilePath            mstypes.RPCUnicodeString
	HomeDirectory          mstypes.RPCUnicodeString
	HomeDirectoryDrive     mstypes.RPCUnicodeString
	LogonCount             uint16
	BadPasswordCount       uint16
	UserID                 uint32
	PrimaryGroupID         uint32
	GroupCount             uint32
	GroupIDs               []mstypes.GroupMembership `ndr:"pointer,conformant"`
	UserFlags              uint32
	UserSessionKey         mstypes.UserSessionKey
	LogonServer            mstypes.RPCUnicodeString
	LogonDomainName        mstypes.RPCUnicodeString
	LogonDomainID          mstypes.RPCSID `ndr:"pointer"`
	Reserved1              [2]uint32      // Has 2 elements
	UserAccountControl     uint32
	SubAuthStatus          uint32
	LastSuccessfulILogon   mstypes.FileTime
	LastFailedILogon       mstypes.FileTime
	FailedILogonCount      uint32
	Reserved3              uint32
	SIDCount               uint32
	ExtraSIDs              []mstypes.KerbSidAndAttributes `ndr:"pointer,conformant"`
	ResourceGroupDomainSID mstypes.RPCSID                 `ndr:"pointer"`
	ResourceGroupCount     uint32
	ResourceGroupIDs       []mstypes.GroupMembership `ndr:"pointer,conformant"`
}
