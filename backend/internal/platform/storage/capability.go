package storage

type Capabilities struct {
	RangeRead       bool
	AtomicPublish   bool
	VerifiedCopy    bool
	Multipart       bool
	PresignedUpload bool
	POSIX           bool
}
