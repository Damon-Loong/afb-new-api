package common

import (
	"bytes"
	"io"

	basecommon "github.com/QuantumNous/new-api/common"
)

type noopCloser struct{}

func (noopCloser) Close() error {
	return nil
}

// NewOutboundJSONBody keeps ordinary JSON requests on an allocation-free,
// lock-free bytes.Reader path. BodyStorage is only used when disk caching is
// enabled and the payload is large enough to actually be written to disk.
func NewOutboundJSONBody(data []byte) (body io.Reader, size int64, closer io.Closer, err error) {
	size = int64(len(data))
	if !basecommon.ShouldUseDiskCache(size) {
		return bytes.NewReader(data), size, noopCloser{}, nil
	}

	storage, err := basecommon.CreateBodyStorage(data)
	if err != nil {
		return nil, 0, nil, err
	}
	return basecommon.ReaderOnly(storage), storage.Size(), storage, nil
}
