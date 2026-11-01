package files

// Visibility is a storage-agnostic access mode for an object.
// Adapters must not assume a public HTTP URL exists.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate:
		return true
	default:
		return false
	}
}

func (v Visibility) Normalize() Visibility {
	if v == VisibilityPrivate {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

// Purpose describes why a file was uploaded. Storage adapters ignore it;
// the product layer uses it for validation and ACL.
type Purpose string

const (
	PurposeImage       Purpose = "image"
	PurposeCertificate Purpose = "certificate"
)

func (p Purpose) IsValid() bool {
	switch p {
	case PurposeImage, PurposeCertificate, "":
		return true
	default:
		return false
	}
}

func (p Purpose) Normalize() Purpose {
	if p == PurposeCertificate {
		return PurposeCertificate
	}
	return PurposeImage
}
