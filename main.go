package main

import (
	"mime"

	"github.com/versioneer-tech/package-r/cmd"
)

func main() {
	// avoid mime type probes (readFirstBytes...)
	mime.AddExtensionType(".npy", "application/octet-stream")
	mime.AddExtensionType(".npz", "application/zip")
	mime.AddExtensionType(".parquet", "application/x-parquet")
	mime.AddExtensionType(".arrow", "application/vnd.apache.arrow.file")
	mime.AddExtensionType(".feather", "application/vnd.apache.arrow.file")
	mime.AddExtensionType(".orc", "application/vnd.apache.orc")
	mime.AddExtensionType(".avro", "application/avro")
	mime.AddExtensionType(".zarr", "application/x-zarr")
	mime.AddExtensionType(".cog", "image/tiff")
	mime.AddExtensionType(".cbk", "application/x-netcdf")
	mime.AddExtensionType(".h5", "application/x-hdf5")
	mime.AddExtensionType(".hdf", "application/x-hdf")
	mime.AddExtensionType(".nc", "application/x-netcdf")
	mime.AddExtensionType(".grib", "application/x-grib")
	mime.AddExtensionType(".grb", "application/x-grib")
	mime.AddExtensionType(".geojson", "application/geo+json")
	mime.AddExtensionType(".stac", "application/json")
	mime.AddExtensionType(".yaml", "application/x-yaml")
	mime.AddExtensionType(".yml", "application/x-yaml")
	mime.AddExtensionType(".7z", "application/x-7z-compressed")
	mime.AddExtensionType(".env", "text/plain")
	mime.AddExtensionType(".ini", "text/plain")
	mime.AddExtensionType(".toml", "text/plain")
	mime.AddExtensionType(".md", "text/markdown")
	mime.AddExtensionType(".log", "text/plain")
	mime.AddExtensionType(".conf", "text/plain")
	mime.AddExtensionType(".sql", "application/sql")
	mime.AddExtensionType(".csv", "text/csv")
	mime.AddExtensionType(".tsv", "text/tab-separated-values")
	mime.AddExtensionType(".rst", "text/plain")
	mime.AddExtensionType(".py", "text/x-python")
	mime.AddExtensionType(".sh", "text/x-shellscript")
	mime.AddExtensionType(".go", "text/x-go")
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".ts", "application/javascript")
	mime.AddExtensionType(".html", "text/html")
	cmd.Execute()
}
