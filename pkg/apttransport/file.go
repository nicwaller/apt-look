package apttransport

import "context"

type FileTransport struct{}

//goland:noinspection GoUnusedExportedFunction
func NewFileTransport() *FileTransport {
	return &FileTransport{}
}

func (t *FileTransport) Schemes() []string {
	return []string{"file"}
}

func (t *FileTransport) Acquire(ctx context.Context, req *AcquireRequest) (*AcquireResponse, error) {
	// TODO: Implement file-specific logic
	// - Open local file
	// - Check modification time
	// - Calculate hashes if needed
	// - Copy to destination if filename specified

	return nil, &AcquireError{
		URI:    req.URI,
		Reason: "not implemented",
		Err:    nil,
	}
}

func (t *FileTransport) Close() error {
	return nil
}
