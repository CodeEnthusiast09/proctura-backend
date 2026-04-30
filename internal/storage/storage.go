package storage

// CloudinaryMaxBytes is the file size threshold above which uploads are routed to MinIO.
// Cloudinary's free plan caps individual video uploads at 100 MB.
const CloudinaryMaxBytes int64 = 100 * 1024 * 1024

// UploadToken is the provider-agnostic token returned to the frontend.
type UploadToken struct {
	Provider   string           `json:"provider"`
	Cloudinary *CloudinaryToken `json:"cloudinary,omitempty"`
	MinIO      *MinIOToken      `json:"minio,omitempty"`
}

type CloudinaryToken struct {
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
	APIKey    string `json:"api_key"`
	CloudName string `json:"cloud_name"`
	Folder    string `json:"folder"`
}

type MinIOToken struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
}

// Router picks a storage provider based on file size.
type Router struct {
	cloudinary cloudinaryProvider
	minio      *minioProvider // nil when MinIO is not configured
}

func NewRouter(c cloudinaryProvider, m *minioProvider) *Router {
	return &Router{cloudinary: c, minio: m}
}

// Token returns an upload token for a given submission and file size.
// Files >= CloudinaryMaxBytes are routed to MinIO (if configured).
func (r *Router) Token(submissionID string, sizeBytes int64) (*UploadToken, error) {
	if r.minio != nil && sizeBytes >= CloudinaryMaxBytes {
		return r.minio.token(submissionID)
	}
	return r.cloudinary.token()
}
