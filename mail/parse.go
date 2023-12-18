package mail

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
)

// part
// copyable representation of a multipart.Part
type part struct {
	header textproto.MIMEHeader
	body   []byte
}

// parseMIMEParts will recursively walk a MIME entity and return a []mime.Part containing
// each (flattened) mime.Part found.
// note: there are no restrictions on recursion
func parseMIMEParts(hs textproto.MIMEHeader, b io.Reader) ([]*part, error) {
	var ps []*part
	// If no content type is given, set it to the default
	if _, ok := hs[CONTENT_TYPE]; !ok {
		hs.Set(CONTENT_TYPE, DefaultContentType)
	}
	ct, params, err := mime.ParseMediaType(hs.Get(CONTENT_TYPE))
	if err != nil {
		return ps, err
	}
	// If it's a multipart email, recursively parse the parts
	if strings.HasPrefix(ct, MULTIPART) {
		if _, ok := params[BOUNDARY]; !ok {
			return ps, ErrMissingBoundary
		}
		mr := multipart.NewReader(b, params[BOUNDARY])
		for {
			var buf bytes.Buffer
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return ps, err
			}
			if _, ok := p.Header[CONTENT_TYPE]; !ok {
				p.Header.Set(CONTENT_TYPE, DefaultContentType)
			}
			subct, _, err := mime.ParseMediaType(p.Header.Get(CONTENT_TYPE))
			if err != nil {
				return ps, err
			}
			if strings.HasPrefix(subct, MULTIPART) {
				sps, err := parseMIMEParts(p.Header, p)
				if err != nil {
					return ps, err
				}
				ps = append(ps, sps...)
			} else {
				var reader io.Reader
				reader = p
				if p.Header.Get(CONTENT_TRANSFER_ENCODING) == BASE_64 {
					reader = base64.NewDecoder(base64.StdEncoding, reader)
				}
				// Otherwise, just append the part to the list
				// Copy the part data into the buffer
				if _, err := io.Copy(&buf, reader); err != nil {
					return ps, err
				}
				ps = append(ps, &part{body: buf.Bytes(), header: p.Header})
			}
		}
	} else {
		// If it is not a multipart email, parse the body content as a single "part"
		switch hs.Get(CONTENT_TRANSFER_ENCODING) {
		case QUOTED_PRINTABLE:
			b = quotedprintable.NewReader(b)
		case BASE_64:
			b = base64.NewDecoder(base64.StdEncoding, b)
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, b); err != nil {
			return ps, err
		}
		ps = append(ps, &part{body: buf.Bytes(), header: hs})
	}
	return ps, nil
}
