// +build windows,amd64
package winapi

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	so "github.com/iamacarpet/go-win64api/shared"
)

var (
	modNetapi32                = syscall.NewLazyDLL("netapi32.dll")
	usrNetUserEnum             = modNetapi32.NewProc("NetUserEnum")
	usrNetUserAdd              = modNetapi32.NewProc("NetUserAdd")
	usrNetUserDel              = modNetapi32.NewProc("NetUserDel")
	usrNetGetAnyDCName         = modNetapi32.NewProc("NetGetAnyDCName")
	usrNetUserGetInfo          = modNetapi32.NewProc("NetUserGetInfo")
	usrNetUserSetInfo          = modNetapi32.NewProc("NetUserSetInfo")
	usrNetLocalGroupAddMembers = modNetapi32.NewProc("NetLocalGroupAddMembers")
	usrNetLocalGroupDelMembers = modNetapi32.NewProc("NetLocalGroupDelMembers")
	usrNetApiBufferFree        = modNetapi32.NewProc("NetApiBufferFree")
)

const (
	NET_API_STATUS_NERR_Success                      = 0
	NET_API_STATUS_NERR_InvalidComputer              = 2351
	NET_API_STATUS_NERR_NotPrimary                   = 2226
	NET_API_STATUS_NERR_SpeGroupOp                   = 2234
	NET_API_STATUS_NERR_LastAdmin                    = 2452
	NET_API_STATUS_NERR_BadPassword                  = 2203
	NET_API_STATUS_NERR_PasswordTooShort             = 2245
	NET_API_STATUS_NERR_UserNotFound                 = 2221
	NET_API_STATUS_ERROR_ACCESS_DENIED               = 5
	NET_API_STATUS_ERROR_NOT_ENOUGH_MEMORY           = 8
	NET_API_STATUS_ERROR_INVALID_PARAMETER           = 87
	NET_API_STATUS_ERROR_INVALID_NAME                = 123
	NET_API_STATUS_ERROR_INVALID_LEVEL               = 124
	NET_API_STATUS_ERROR_MORE_DATA                   = 234
	NET_API_STATUS_ERROR_SESSION_CREDENTIAL_CONFLICT = 1219
	NET_API_STATUS_RPC_S_SERVER_UNAVAILABLE          = 2147944122
	NET_API_STATUS_RPC_E_REMOTE_DISABLED             = 2147549468

	USER_PRIV_MASK  = 0x3
	USER_PRIV_GUEST = 0
	USER_PRIV_USER  = 1
	USER_PRIV_ADMIN = 2

	USER_FILTER_NORMAL_ACCOUNT = 0x0002
	USER_MAX_PREFERRED_LENGTH  = 0xFFFFFFFF

	USER_UF_SCRIPT             = 1
	USER_UF_ACCOUNTDISABLE     = 2
	USER_UF_LOCKOUT            = 16
	USER_UF_PASSWD_CANT_CHANGE = 64
	USER_UF_NORMAL_ACCOUNT     = 512
	USER_UF_DONT_EXPIRE_PASSWD = 65536
)

type USER_INFO_1 struct {
	Usri1_name         *uint16
	Usri1_password     *uint16
	Usri1_password_age uint32
	Usri1_priv         uint32
	Usri1_home_dir     *uint16
	Usri1_comment      *uint16
	Usri1_flags        uint32
	Usri1_script_path  *uint16
}

type USER_INFO_2 struct {
	Usri2_name           *uint16
	Usri2_password       *uint16
	Usri2_password_age   uint32
	Usri2_priv           uint32
	Usri2_home_dir       *uint16
	Usri2_comment        *uint16
	Usri2_flags          uint32
	Usri2_script_path    *uint16
	Usri2_auth_flags     uint32
	Usri2_full_name      *uint16
	Usri2_usr_comment    *uint16
	Usri2_parms          *uint16
	Usri2_workstations   *uint16
	Usri2_last_logon     uint32
	Usri2_last_logoff    uint32
	Usri2_acct_expires   uint32
	Usri2_max_storage    uint32
	Usri2_units_per_week uint32
	Usri2_logon_hours    uintptr
	Usri2_bad_pw_count   uint32
	Usri2_num_logons     uint32
	Usri2_logon_server   *uint16
	Usri2_country_code   uint32
	Usri2_code_page      uint32
}

type USER_INFO_1003 struct {
	Usri1003_password *uint16
}

type USER_INFO_1008 struct {
	Usri1008_flags uint32
}

type USER_INFO_1011 struct {
	Usri1011_full_name *uint16
}

type LOCALGROUP_MEMBERS_INFO_3 struct {
	Lgrmi3_domainandname *uint16
}

func UserAdd(username string, fullname string, password string) (bool, error) {
	var parmErr uint32
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	pPointer, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return false, fmt.Errorf("Unable to encode password to UTF16")
	}
	uInfo := USER_INFO_1{
		Usri1_name:     uPointer,
		Usri1_password: pPointer,
		Usri1_priv:     USER_PRIV_USER,
		Usri1_flags:    USER_UF_SCRIPT | USER_UF_NORMAL_ACCOUNT | USER_UF_DONT_EXPIRE_PASSWD,
	}
	ret, _, _ := usrNetUserAdd.Call(
		uintptr(0),
		uintptr(uint32(1)),
		uintptr(unsafe.Pointer(&uInfo)),
		uintptr(unsafe.Pointer(&parmErr)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d %d", ret, parmErr)
	}
	if ok, err := UserUpdateFullname(username, fullname); err != nil {
		return false, fmt.Errorf("While setting Full Name. %s", err.Error())
	} else if !ok {
		return false, fmt.Errorf("Problem while setting Full Name.")
	}

	return AddGroupMembership(username, "Users")
}

func UserDelete(username string) (bool, error) {
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	ret, _, _ := usrNetUserDel.Call(
		uintptr(0),
		uintptr(unsafe.Pointer(uPointer)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func IsLocalUserAdmin(username string) (bool, error) {
	var dataPointer uintptr
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	_, _, _ = usrNetUserGetInfo.Call(
		uintptr(0),                            // servername
		uintptr(unsafe.Pointer(uPointer)),     // username
		uintptr(uint32(1)),                    // level, request USER_INFO_1
		uintptr(unsafe.Pointer(&dataPointer)), // Pointer to struct.
	)
	defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dataPointer)))

	if dataPointer == uintptr(0) {
		return false, fmt.Errorf("Unable to get data structure.")
	}

	var data *USER_INFO_1 = (*USER_INFO_1)(unsafe.Pointer(dataPointer))

	if data.Usri1_priv == USER_PRIV_ADMIN {
		return true, nil
	} else {
		return false, nil
	}
}

func IsDomainUserAdmin(username string, domain string) (bool, error) {
	var dataPointer uintptr
	var dcPointer uintptr
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	dPointer, err := syscall.UTF16PtrFromString(domain)
	if err != nil {
		return false, fmt.Errorf("Unable to encode domain to UTF16")
	}

	_, _, _ = usrNetGetAnyDCName.Call(
		uintptr(0),                        // servername
		uintptr(unsafe.Pointer(dPointer)), // domainame
		uintptr(unsafe.Pointer(&dcPointer)),
	)
	defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dcPointer)))

	_, _, _ = usrNetUserGetInfo.Call(
		uintptr(dcPointer),                    // servername
		uintptr(unsafe.Pointer(uPointer)),     // username
		uintptr(uint32(1)),                    // level, request USER_INFO_1
		uintptr(unsafe.Pointer(&dataPointer)), // Pointer to struct.
	)
	defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dataPointer)))

	if dataPointer == uintptr(0) {
		return false, fmt.Errorf("Unable to get data structure.")
	}

	var data *USER_INFO_1 = (*USER_INFO_1)(unsafe.Pointer(dataPointer))

	if data.Usri1_priv == USER_PRIV_ADMIN {
		return true, nil
	} else {
		return false, nil
	}
}

func ListLocalUsers() ([]so.LocalUser, error) {
	var (
		dataPointer  uintptr
		resumeHandle uintptr
		entriesRead  uint32
		entriesTotal uint32
		sizeTest     USER_INFO_2
		retVal       []so.LocalUser = make([]so.LocalUser, 0)
	)

	ret, _, _ := usrNetUserEnum.Call(
		uintptr(0),         // servername
		uintptr(uint32(2)), // level, USER_INFO_2
		uintptr(uint32(USER_FILTER_NORMAL_ACCOUNT)), // filter, only "normal" accounts.
		uintptr(unsafe.Pointer(&dataPointer)),       // struct buffer for output data.
		uintptr(uint32(USER_MAX_PREFERRED_LENGTH)),  // allow as much memory as required.
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&entriesTotal)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return nil, fmt.Errorf("Error fetching user entry.")
	} else if dataPointer == uintptr(0) {
		return nil, fmt.Errorf("Null pointer while fetching entry.")
	}

	var iter uintptr = dataPointer
	for i := uint32(0); i < entriesRead; i++ {
		var data *USER_INFO_2 = (*USER_INFO_2)(unsafe.Pointer(iter))

		ud := so.LocalUser{
			Username:         UTF16toString(data.Usri2_name),
			FullName:         UTF16toString(data.Usri2_full_name),
			PasswordAge:      (time.Duration(data.Usri2_password_age) * time.Second),
			LastLogon:        time.Unix(int64(data.Usri2_last_logon), 0),
			BadPasswordCount: data.Usri2_bad_pw_count,
			NumberOfLogons:   data.Usri2_num_logons,
		}

		if (data.Usri2_flags & USER_UF_ACCOUNTDISABLE) != USER_UF_ACCOUNTDISABLE {
			ud.IsEnabled = true
		}
		if (data.Usri2_flags & USER_UF_LOCKOUT) == USER_UF_LOCKOUT {
			ud.IsLocked = true
		}
		if (data.Usri2_flags & USER_UF_PASSWD_CANT_CHANGE) == USER_UF_PASSWD_CANT_CHANGE {
			ud.NoChangePassword = true
		}
		if (data.Usri2_flags & USER_UF_DONT_EXPIRE_PASSWD) == USER_UF_DONT_EXPIRE_PASSWD {
			ud.PasswordNeverExpires = true
		}
		if data.Usri2_priv == USER_PRIV_ADMIN {
			ud.IsAdmin = true
		}

		retVal = append(retVal, ud)

		iter = uintptr(unsafe.Pointer(iter + unsafe.Sizeof(sizeTest)))
	}
	_, _, _ = usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dataPointer)))
	return retVal, nil
}

func AddGroupMembership(username, groupname string) (bool, error) {
	hn, _ := os.Hostname()
	uPointer, err := syscall.UTF16PtrFromString(hn + `\` + username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	gPointer, err := syscall.UTF16PtrFromString(groupname)
	if err != nil {
		return false, fmt.Errorf("Unable to encode group name to UTF16")
	}
	var uArray []LOCALGROUP_MEMBERS_INFO_3 = make([]LOCALGROUP_MEMBERS_INFO_3, 1)
	uArray[0] = LOCALGROUP_MEMBERS_INFO_3{
		Lgrmi3_domainandname: uPointer,
	}
	ret, _, _ := usrNetLocalGroupAddMembers.Call(
		uintptr(0),                          // servername
		uintptr(unsafe.Pointer(gPointer)),   // group name
		uintptr(uint32(3)),                  // level
		uintptr(unsafe.Pointer(&uArray[0])), // user array.
		uintptr(uint32(len(uArray))),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func RemoveGroupMembership(username, groupname string) (bool, error) {
	hn, _ := os.Hostname()
	uPointer, err := syscall.UTF16PtrFromString(hn + `\` + username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	gPointer, err := syscall.UTF16PtrFromString(groupname)
	if err != nil {
		return false, fmt.Errorf("Unable to encode group name to UTF16")
	}
	var uArray []LOCALGROUP_MEMBERS_INFO_3 = make([]LOCALGROUP_MEMBERS_INFO_3, 1)
	uArray[0] = LOCALGROUP_MEMBERS_INFO_3{
		Lgrmi3_domainandname: uPointer,
	}
	ret, _, _ := usrNetLocalGroupDelMembers.Call(
		uintptr(0),                          // servername
		uintptr(unsafe.Pointer(gPointer)),   // group name
		uintptr(uint32(3)),                  // level
		uintptr(unsafe.Pointer(&uArray[0])), // user array.
		uintptr(uint32(len(uArray))),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func SetAdmin(username string) (bool, error) {
	return AddGroupMembership(username, "Administrators")
}

func RevokeAdmin(username string) (bool, error) {
	return RemoveGroupMembership(username, "Administrators")
}

func UserUpdateFullname(username string, fullname string) (bool, error) {
	var errParam uint32
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	fPointer, err := syscall.UTF16PtrFromString(fullname)
	if err != nil {
		return false, fmt.Errorf("Unable to encode full name to UTF16")
	}
	ret, _, _ := usrNetUserSetInfo.Call(
		uintptr(0),                        // servername
		uintptr(unsafe.Pointer(uPointer)), // username
		uintptr(uint32(1011)),             // level
		uintptr(unsafe.Pointer(&USER_INFO_1011{Usri1011_full_name: fPointer})),
		uintptr(unsafe.Pointer(&errParam)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func ChangePassword(username string, password string) (bool, error) {
	var errParam uint32
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	pPointer, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	ret, _, _ := usrNetUserSetInfo.Call(
		uintptr(0),                        // servername
		uintptr(unsafe.Pointer(uPointer)), // username
		uintptr(uint32(1003)),             // level
		uintptr(unsafe.Pointer(&USER_INFO_1003{Usri1003_password: pPointer})),
		uintptr(unsafe.Pointer(&errParam)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func UserDisabled(username string, disable bool) (bool, error) {
	if disable {
		return userAddFlags(username, USER_UF_ACCOUNTDISABLE)
	} else {
		return userDelFlags(username, USER_UF_ACCOUNTDISABLE)
	}
}

func UserPasswordNoExpires(username string, noexpire bool) (bool, error) {
	if noexpire {
		return userAddFlags(username, USER_UF_DONT_EXPIRE_PASSWD)
	} else {
		return userDelFlags(username, USER_UF_DONT_EXPIRE_PASSWD)
	}
}

func UserDisablePasswordChange(username string, disabled bool) (bool, error) {
	if disabled {
		return userAddFlags(username, USER_UF_PASSWD_CANT_CHANGE)
	} else {
		return userDelFlags(username, USER_UF_PASSWD_CANT_CHANGE)
	}
}

func userGetFlags(username string) (uint32, error) {
	var dataPointer uintptr
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return 0, fmt.Errorf("Unable to encode username to UTF16")
	}
	_, _, _ = usrNetUserGetInfo.Call(
		uintptr(0),                            // servername
		uintptr(unsafe.Pointer(uPointer)),     // username
		uintptr(uint32(1)),                    // level, request USER_INFO_1
		uintptr(unsafe.Pointer(&dataPointer)), // Pointer to struct.
	)
	defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dataPointer)))

	if dataPointer == uintptr(0) {
		return 0, fmt.Errorf("Unable to get data structure.")
	}

	var data *USER_INFO_1 = (*USER_INFO_1)(unsafe.Pointer(dataPointer))

	fmt.Printf("Existing user flags: %d\r\n", data.Usri1_flags)
	return data.Usri1_flags, nil
}

func userSetFlags(username string, flags uint32) (bool, error) {
	var errParam uint32
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}
	ret, _, _ := usrNetUserSetInfo.Call(
		uintptr(0),                        // servername
		uintptr(unsafe.Pointer(uPointer)), // username
		uintptr(uint32(1008)),             // level
		uintptr(unsafe.Pointer(&USER_INFO_1008{Usri1008_flags: flags})),
		uintptr(unsafe.Pointer(&errParam)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, fmt.Errorf("Unable to process. %d", ret)
	}
	return true, nil
}

func userAddFlags(username string, flags uint32) (bool, error) {
	eFlags, err := userGetFlags(username)
	if err != nil {
		return false, fmt.Errorf("Error while getting existing flags, %s.", err.Error())
	}
	eFlags |= flags // add supplied bits to mask.
	return userSetFlags(username, eFlags)
}

func userDelFlags(username string, flags uint32) (bool, error) {
	eFlags, err := userGetFlags(username)
	if err != nil {
		return false, fmt.Errorf("Error while getting existing flags, %s.", err.Error())
	}
	eFlags &^= flags // clear bits we want to remove.
	return userSetFlags(username, eFlags)
}

func UTF16toString(p *uint16) string {
	return syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(p))[:])
}

func DomainUserLocked(username string, domain string) (bool, error) {
	var dataPointer uintptr
	//var dcPointer uintptr
	var servername uintptr

	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	if domain != "" {
		dPointer, err := syscall.UTF16PtrFromString(domain)
		if err != nil {
			return false, fmt.Errorf("Unable to encode domain to UTF16")
		}
		servername = uintptr(unsafe.Pointer(dPointer))
	} else {
		servername = uintptr(0)
	}

	_, _, _ = usrNetUserGetInfo.Call(
		servername,                            // servername
		uintptr(unsafe.Pointer(uPointer)),     // username
		uintptr(uint32(2)),                    // level, request USER_INFO_2
		uintptr(unsafe.Pointer(&dataPointer)), // Pointer to struct.
	)
	defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(dataPointer)))

	if dataPointer == uintptr(0) {
		return false, fmt.Errorf("Unable to get data structure.")
	}

	data := (*USER_INFO_2)(unsafe.Pointer(dataPointer))

	return (data.Usri2_flags & USER_UF_LOCKOUT) == USER_UF_LOCKOUT, nil
}
