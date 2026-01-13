package errors

import (
	"fmt"
)

type ErrorType string

type Error struct {
	Type ErrorType
	Info string
}

func (e Error) Error() string {
	return fmt.Sprintf("Error - Type: %s, Info: %s", e.Type, e.Info)
}

const (
	KeyError        ErrorType = "KeyError"        // 由特定的键值引发的错误
	AttributeError  ErrorType = "AttributeError"  // 由属性或配置引发的错误
	IndexError      ErrorType = "IndexError"      // 由索引操作引发的错误
	NetworkError    ErrorType = "NetworkError"    // 由网络通信引发的错误
	ImportError     ErrorType = "ImportError"     // 引用的资源不合法或未找到引发的错误
	InvalidArgument ErrorType = "InvalidArgument" // 由无效参数引发的错误
	ObjectNotFound  ErrorType = "ObjectNotFound"  // 由未找到的对象引发的错误
	OSError         ErrorType = "OSError"         // 由操作系统相关操作引发的错误
	ConditionError  ErrorType = "ConditionError"  // 特定条件不满足时引发的错误
	InternalError   ErrorType = "InternalError"   // 内部逻辑引发的错误
	OperationError  ErrorType = "OperationError"  // 操作执行失败引发的错误
)
