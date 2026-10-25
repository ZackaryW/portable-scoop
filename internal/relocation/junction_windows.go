package relocation

import (
	"encoding/binary"
	"fmt"
	"os"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func createJunction(path, target string) error {
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(pathPointer, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	substitute := utf16.Encode([]rune(`\??\` + target))
	printName := utf16.Encode([]rune(target))
	buffer := make([]byte, 16+2*(len(substitute)+len(printName)+2))
	binary.LittleEndian.PutUint32(buffer[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:], uint16(len(substitute)*2))
	binary.LittleEndian.PutUint16(buffer[12:], uint16((len(substitute)+1)*2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16(len(printName)*2))
	for index, value := range append(append(substitute, 0), append(printName, 0)...) {
		binary.LittleEndian.PutUint16(buffer[16+index*2:], value)
	}
	var returned uint32
	if err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, (*byte)(unsafe.Pointer(&buffer[0])), uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		return fmt.Errorf("create managed junction: %w", err)
	}
	return nil
}

func removeLink(path string) error {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err = windows.SetFileAttributes(pointer, windows.FILE_ATTRIBUTE_NORMAL); err != nil {
		return err
	}
	return os.Remove(path)
}
