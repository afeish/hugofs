package global

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal

// RpcErrCode is only for identification of the error.
// It is not a gRPC status code, it is a complement of
// the grpc status code to help clients handle the grpc
// error gracefully.
/** ENUM(
      INVALID,
	  NOT_A_DIRECTORY,
	  NOT_A_REGULAR_FILE,
	  INVALID_ENTRY_NAME,
)
*/
type RpcErrCode int
