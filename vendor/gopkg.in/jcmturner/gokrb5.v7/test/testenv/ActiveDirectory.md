# Active Directory Test Environment Setup Notes


## Claims
* Needs Windows 2012
### Enable Claims
* Administrative Tools > Group Policy Management
  * Forest > Domains > DOMAIN.COM > Default Domain Policy (right click, Edit)
  * Compute Configuration > Policies > Administrative Templates > System > KDC
    * Edit "KDC Support for claims"
    * Set to "Enabled" with the option "Always provide claims"
    
### Configure Claims Values
* Administrative Tools > Active Directory Administrative Center
  * Dynamic Access Control > Claim Types > New

| Display name | Attribute | Type |
| -------------|-----------|------|
| username | sAMAccountName | string |
| otherIpPhone | otherIpPhone | multi-valued string |
| objectClass | objectClass | multi-valued unsigned integer |
| msDS-SupportedEncryptionTypes | msDS-SupportedEncryptionTypes | Integer |

### Add Attributes to User
* Edit testuser1 in Active Directory Users and Computers
* Go to Telephones tab
* Click the "Other" button next to IP Phone
* Add these strings:
  * str1
  * str2
  * str3
  * str4

### Inspect Values
```
Get-ADUser -Filter 'Name -like "*test*1*" -properties *
```
    