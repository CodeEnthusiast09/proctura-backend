package storage

import "github.com/CodeEnthusiast09/proctura-backend/internal/cloudinary"

type cloudinaryProvider struct {
	client *cloudinary.Client
}

func NewCloudinaryProvider(client *cloudinary.Client) cloudinaryProvider {
	return cloudinaryProvider{client: client}
}

func (p cloudinaryProvider) token() (*UploadToken, error) {
	sig := p.client.Sign()
	return &UploadToken{
		Provider: "cloudinary",
		Cloudinary: &CloudinaryToken{
			Timestamp: sig.Timestamp,
			Signature: sig.Signature,
			APIKey:    sig.APIKey,
			CloudName: sig.CloudName,
			Folder:    sig.Folder,
		},
	}, nil
}
