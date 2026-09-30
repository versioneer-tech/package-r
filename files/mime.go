package files

import (
	"fmt"
	"mime"
)

const (
	// ContentBinaryHeaderValue is the content type for binary data.
	ContentBinaryHeaderValue = "application/octet-stream"
	// ContentWebassemblyHeaderValue is the content type for WebAssembly data.
	ContentWebassemblyHeaderValue = "application/wasm"
	// ContentHTMLHeaderValue is the content type for HTML data.
	ContentHTMLHeaderValue = "text/html"
	// ContentJSONHeaderValue is the content type for JSON data.
	ContentJSONHeaderValue = "application/json"
	// ContentJSONProblemHeaderValue is the content type for JSON problem details.
	ContentJSONProblemHeaderValue = "application/problem+json"
	// ContentXMLProblemHeaderValue is the content type for XML problem details.
	ContentXMLProblemHeaderValue = "application/problem+xml"
	// ContentJavascriptHeaderValue is the content type for JavaScript data.
	ContentJavascriptHeaderValue = "text/javascript"
	// ContentTextHeaderValue is the content type for plain text data.
	ContentTextHeaderValue = "text/plain"
	// ContentXMLHeaderValue is the content type for XML data.
	ContentXMLHeaderValue = "application/xml"
	// ContentXMLUnreadableHeaderValue is kept for compatibility.
	ContentXMLUnreadableHeaderValue = ContentXMLHeaderValue
	// ContentMarkdownHeaderValue is the content type for Markdown data.
	ContentMarkdownHeaderValue = "text/markdown"
	// ContentYAMLHeaderValue is the content type for YAML data.
	ContentYAMLHeaderValue = "application/yaml"
	// ContentYAMLTextHeaderValue is kept for compatibility.
	ContentYAMLTextHeaderValue = ContentYAMLHeaderValue
	// ContentProtobufHeaderValue is the content type for Protobuf data.
	ContentProtobufHeaderValue = "application/x-protobuf"
	// ContentMsgPackHeaderValue is the content type for MessagePack data.
	ContentMsgPackHeaderValue = "application/msgpack"
	// ContentMsgPack2HeaderValue is an alternative content type for MessagePack data.
	ContentMsgPack2HeaderValue = "application/x-msgpack"
	// ContentFormHeaderValue is the content type for form data.
	ContentFormHeaderValue = "application/x-www-form-urlencoded"
	// ContentFormMultipartHeaderValue is the content type for multipart form data.
	ContentFormMultipartHeaderValue = "multipart/form-data"
	// ContentMultipartRelatedHeaderValue is the content type for related multipart data.
	ContentMultipartRelatedHeaderValue = "multipart/related"
	// ContentGRPCHeaderValue is the content type for gRPC data.
	ContentGRPCHeaderValue = "application/grpc"
)

// extensionTypes contains mappings that package-r needs to classify and preview
// files consistently on all operating systems. The Go standard library and the
// operating system MIME database handle other file types.
var extensionTypes = map[string]string{
	// Text and source files.
	".cjs":      ContentJavascriptHeaderValue,
	".conf":     ContentTextHeaderValue,
	".css":      "text/css",
	".csv":      "text/csv",
	".env":      ContentTextHeaderValue,
	".geojson":  "application/geo+json",
	".go":       "text/x-go",
	".htm":      ContentHTMLHeaderValue,
	".html":     ContentHTMLHeaderValue,
	".ini":      ContentTextHeaderValue,
	".js":       ContentJavascriptHeaderValue,
	".json":     ContentJSONHeaderValue,
	".jsonl":    "application/x-ndjson",
	".log":      ContentTextHeaderValue,
	".markdown": ContentMarkdownHeaderValue,
	".md":       ContentMarkdownHeaderValue,
	".mjs":      ContentJavascriptHeaderValue,
	".ndjson":   "application/x-ndjson",
	".py":       "text/x-python",
	".rst":      ContentTextHeaderValue,
	".sh":       "text/x-shellscript",
	".sql":      "application/sql",
	".stac":     ContentJSONHeaderValue,
	".toml":     ContentTextHeaderValue,
	".ts":       ContentJavascriptHeaderValue,
	".tsv":      "text/tab-separated-values",
	".txt":      ContentTextHeaderValue,
	".wasm":     ContentWebassemblyHeaderValue,
	".xml":      ContentXMLHeaderValue,
	".yaml":     ContentYAMLHeaderValue,
	".yml":      ContentYAMLHeaderValue,

	// Documents and fonts.
	".otf":   "font/otf",
	".pdf":   "application/pdf",
	".woff":  "font/woff",
	".woff2": "font/woff2",

	// Images.
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".cog":  "image/tiff",
	".gif":  "image/gif",
	".heic": "image/heic",
	".heif": "image/heif",
	".ico":  "image/vnd.microsoft.icon",
	".jpe":  "image/jpeg",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
	".webp": "image/webp",

	// Audio and video.
	".aac":  "audio/aac",
	".avi":  "video/x-msvideo",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".m4v":  "video/mp4",
	".mid":  "audio/midi",
	".midi": "audio/midi",
	".mkv":  "video/x-matroska",
	".mov":  "video/quicktime",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".oga":  "audio/ogg",
	".ogg":  "audio/ogg",
	".ogv":  "video/ogg",
	".opus": "audio/ogg",
	".wav":  "audio/wav",
	".webm": "video/webm",

	// Data and cloud-native formats. These mappings also prevent content probes.
	".arrow":   "application/vnd.apache.arrow.file",
	".avro":    "application/avro",
	".cbk":     "application/x-netcdf",
	".feather": "application/vnd.apache.arrow.file",
	".grb":     "application/x-grib",
	".grib":    "application/x-grib",
	".h5":      "application/x-hdf5",
	".hdf":     "application/x-hdf",
	".nc":      "application/x-netcdf",
	".npy":     ContentBinaryHeaderValue,
	".npz":     "application/zip",
	".orc":     "application/vnd.apache.orc",
	".parquet": "application/vnd.apache.parquet",
	".zarr":    "application/x-zarr",

	// Archives and packages.
	".7z":  "application/x-7z-compressed",
	".gz":  "application/gzip",
	".rpm": "application/x-rpm",
	".tar": "application/x-tar",
	".zip": "application/zip",
}

//nolint:gochecknoinits
func init() {
	for extension, mediaType := range extensionTypes {
		if err := mime.AddExtensionType(extension, mediaType); err != nil {
			panic(fmt.Sprintf("register MIME type %q for %q: %v", mediaType, extension, err))
		}
	}
}
