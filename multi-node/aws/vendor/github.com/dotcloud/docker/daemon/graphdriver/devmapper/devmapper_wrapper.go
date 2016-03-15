// +build linux,amd64

package devmapper

/*
#cgo LDFLAGS: -L. -ldevmapper
#include <libdevmapper.h>
#include <linux/loop.h> // FIXME: present only for defines, maybe we can remove it?
#include <linux/fs.h>   // FIXME: present only for BLKGETSIZE64, maybe we can remove it?

#ifndef LOOP_CTL_GET_FREE
  #define LOOP_CTL_GET_FREE 0x4C82
#endif

#ifndef LO_FLAGS_PARTSCAN
  #define LO_FLAGS_PARTSCAN 8
#endif

// FIXME: Can't we find a way to do the logging in pure Go?
extern void DevmapperLogCallback(int level, char *file, int line, int dm_errno_or_class, char *str);

static void	log_cb(int level, const char *file, int line, int dm_errno_or_class, const char *f, ...)
{
  char buffer[256];
  va_list ap;

  va_start(ap, f);
  vsnprintf(buffer, 256, f, ap);
  va_end(ap);

  DevmapperLogCallback(level, (char *)file, line, dm_errno_or_class, buffer);
}

static void	log_with_errno_init()
{
  dm_log_with_errno_init(log_cb);
}
*/
import "C"

import (
	"unsafe"
)

type (
	CDmTask C.struct_dm_task

	CLoopInfo64 C.struct_loop_info64
	LoopInfo64  struct {
		loDevice           uint64 /* ioctl r/o */
		loInode            uint64 /* ioctl r/o */
		loRdevice          uint64 /* ioctl r/o */
		loOffset           uint64
		loSizelimit        uint64 /* bytes, 0 == max available */
		loNumber           uint32 /* ioctl r/o */
		loEncrypt_type     uint32
		loEncrypt_key_size uint32 /* ioctl w/o */
		loFlags            uint32 /* ioctl r/o */
		loFileName         [LoNameSize]uint8
		loCryptName        [LoNameSize]uint8
		loEncryptKey       [LoKeySize]uint8 /* ioctl w/o */
		loInit             [2]uint64
	}
)

// IOCTL consts
const (
	BlkGetSize64 = C.BLKGETSIZE64
	BlkDiscard   = C.BLKDISCARD

	LoopSetFd       = C.LOOP_SET_FD
	LoopCtlGetFree  = C.LOOP_CTL_GET_FREE
	LoopGetStatus64 = C.LOOP_GET_STATUS64
	LoopSetStatus64 = C.LOOP_SET_STATUS64
	LoopClrFd       = C.LOOP_CLR_FD
	LoopSetCapacity = C.LOOP_SET_CAPACITY
)

const (
	LoFlagsAutoClear = C.LO_FLAGS_AUTOCLEAR
	LoFlagsReadOnly  = C.LO_FLAGS_READ_ONLY
	LoFlagsPartScan  = C.LO_FLAGS_PARTSCAN
	LoKeySize        = C.LO_KEY_SIZE
	LoNameSize       = C.LO_NAME_SIZE
)

var (
	DmGetLibraryVersion = dmGetLibraryVersionFct
	DmGetNextTarget     = dmGetNextTargetFct
	DmLogInitVerbose    = dmLogInitVerboseFct
	DmSetDevDir         = dmSetDevDirFct
	DmTaskAddTarget     = dmTaskAddTargetFct
	DmTaskCreate        = dmTaskCreateFct
	DmTaskDestroy       = dmTaskDestroyFct
	DmTaskGetInfo       = dmTaskGetInfoFct
	DmTaskRun           = dmTaskRunFct
	DmTaskSetAddNode    = dmTaskSetAddNodeFct
	DmTaskSetCookie     = dmTaskSetCookieFct
	DmTaskSetMessage    = dmTaskSetMessageFct
	DmTaskSetName       = dmTaskSetNameFct
	DmTaskSetRo         = dmTaskSetRoFct
	DmTaskSetSector     = dmTaskSetSectorFct
	DmUdevWait          = dmUdevWaitFct
	LogWithErrnoInit    = logWithErrnoInitFct
)

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func dmTaskDestroyFct(task *CDmTask) {
	C.dm_task_destroy((*C.struct_dm_task)(task))
}

func dmTaskCreateFct(taskType int) *CDmTask {
	return (*CDmTask)(C.dm_task_create(C.int(taskType)))
}

func dmTaskRunFct(task *CDmTask) int {
	ret, _ := C.dm_task_run((*C.struct_dm_task)(task))
	return int(ret)
}

func dmTaskSetNameFct(task *CDmTask, name string) int {
	Cname := C.CString(name)
	defer free(Cname)

	return int(C.dm_task_set_name((*C.struct_dm_task)(task), Cname))
}

func dmTaskSetMessageFct(task *CDmTask, message string) int {
	Cmessage := C.CString(message)
	defer free(Cmessage)

	return int(C.dm_task_set_message((*C.struct_dm_task)(task), Cmessage))
}

func dmTaskSetSectorFct(task *CDmTask, sector uint64) int {
	return int(C.dm_task_set_sector((*C.struct_dm_task)(task), C.uint64_t(sector)))
}

func dmTaskSetCookieFct(task *CDmTask, cookie *uint, flags uint16) int {
	cCookie := C.uint32_t(*cookie)
	defer func() {
		*cookie = uint(cCookie)
	}()
	return int(C.dm_task_set_cookie((*C.struct_dm_task)(task), &cCookie, C.uint16_t(flags)))
}

func dmTaskSetAddNodeFct(task *CDmTask, addNode AddNodeType) int {
	return int(C.dm_task_set_add_node((*C.struct_dm_task)(task), C.dm_add_node_t(addNode)))
}

func dmTaskSetRoFct(task *CDmTask) int {
	return int(C.dm_task_set_ro((*C.struct_dm_task)(task)))
}

func dmTaskAddTargetFct(task *CDmTask,
	start, size uint64, ttype, params string) int {

	Cttype := C.CString(ttype)
	defer free(Cttype)

	Cparams := C.CString(params)
	defer free(Cparams)

	return int(C.dm_task_add_target((*C.struct_dm_task)(task), C.uint64_t(start), C.uint64_t(size), Cttype, Cparams))
}

func dmTaskGetInfoFct(task *CDmTask, info *Info) int {
	Cinfo := C.struct_dm_info{}
	defer func() {
		info.Exists = int(Cinfo.exists)
		info.Suspended = int(Cinfo.suspended)
		info.LiveTable = int(Cinfo.live_table)
		info.InactiveTable = int(Cinfo.inactive_table)
		info.OpenCount = int32(Cinfo.open_count)
		info.EventNr = uint32(Cinfo.event_nr)
		info.Major = uint32(Cinfo.major)
		info.Minor = uint32(Cinfo.minor)
		info.ReadOnly = int(Cinfo.read_only)
		info.TargetCount = int32(Cinfo.target_count)
	}()
	return int(C.dm_task_get_info((*C.struct_dm_task)(task), &Cinfo))
}

func dmGetNextTargetFct(task *CDmTask, next uintptr, start, length *uint64, target, params *string) uintptr {
	var (
		Cstart, Clength      C.uint64_t
		CtargetType, Cparams *C.char
	)
	defer func() {
		*start = uint64(Cstart)
		*length = uint64(Clength)
		*target = C.GoString(CtargetType)
		*params = C.GoString(Cparams)
	}()

	nextp := C.dm_get_next_target((*C.struct_dm_task)(task), unsafe.Pointer(next), &Cstart, &Clength, &CtargetType, &Cparams)
	return uintptr(nextp)
}

func dmUdevWaitFct(cookie uint) int {
	return int(C.dm_udev_wait(C.uint32_t(cookie)))
}

func dmLogInitVerboseFct(level int) {
	C.dm_log_init_verbose(C.int(level))
}

func logWithErrnoInitFct() {
	C.log_with_errno_init()
}

func dmSetDevDirFct(dir string) int {
	Cdir := C.CString(dir)
	defer free(Cdir)

	return int(C.dm_set_dev_dir(Cdir))
}

func dmGetLibraryVersionFct(version *string) int {
	buffer := C.CString(string(make([]byte, 128)))
	defer free(buffer)
	defer func() {
		*version = C.GoString(buffer)
	}()
	return int(C.dm_get_library_version(buffer, 128))
}
