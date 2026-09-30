"""Create a Vienna-only Sentinel-2 STAC fixture.

Rasterio uses HTTP range requests against the source COGs and reads only the
blocks that intersect Vienna. The resulting raster files are local COGs.
Small XML metadata assets are downloaded in full.
"""

from __future__ import annotations

import os
import json
from pathlib import Path
from urllib.parse import urlsplit

import geopandas
import numpy
import rasterio
import requests
import stac_geoparquet.arrow
from pystac_client import Client
from rasterio.enums import ColorInterp
from rasterio.shutil import copy as copy_raster
from rasterio.transform import array_bounds
from rasterio.warp import transform_bounds
from rasterio.windows import Window, from_bounds, transform as window_transform
from shapely.geometry import mapping

STAC_API_URL = "https://earth-search.aws.element84.com/v1"
COLLECTION_ID = "sentinel-2-c1-l2a"
VIENNA_BOUNDARY_URL = "https://data.wien.gv.at/daten/geo"
VIENNA_BBOX = (16.18, 48.11, 16.58, 48.32)
RGB_REFLECTANCE_MAX = 3000
SEARCH_WINDOWS = {
    "mid-February": "2026-02-10T00:00:00Z/2026-02-20T23:59:59Z",
    "mid-May": "2026-05-10T00:00:00Z/2026-05-20T23:59:59Z",
    "mid-August": "2026-08-10T00:00:00Z/2026-08-20T23:59:59Z",
}

REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
FIXTURE_ROOT = REPOSITORY_ROOT / "tests" / "data"
OUTPUT_DIR = FIXTURE_ROOT / "vienna-s2l2a-26"
CATALOG_PATH = OUTPUT_DIR / "vienna-s2l2a-26.parquet"
BOUNDARY_PATH = OUTPUT_DIR / "vienna-boundary.geojson"

RASTER_ASSETS = {
    "red": ("red", "b04"),
    "green": ("green", "b03"),
    "blue": ("blue", "b02"),
    "nir": ("nir", "b08"),
    "cloud": ("cloud", "cloud_probability"),
    "snow": ("snow", "snow_probability"),
    # The visual asset is a georeferenced RGB COG. The preview asset is a JPEG.
    "overview": ("visual",),
}

METADATA_ASSETS = {
    "granule_metadata": ("granule_metadata",),
    "tileinfo_metadata": ("tileinfo_metadata",),
    "product_metadata": ("product_metadata",),
}


def matching_asset(item, candidate_keys: tuple[str, ...]):
    for key in candidate_keys:
        if key in item.assets:
            return key, item.assets[key]
    return None, None


def clipped_window(dataset: rasterio.DatasetReader) -> Window:
    bounds = transform_bounds("EPSG:4326", dataset.crs, *VIENNA_BBOX, densify_pts=21)
    window = from_bounds(*bounds, transform=dataset.transform)
    window = window.round_offsets().round_lengths()
    full_window = Window(0, 0, dataset.width, dataset.height)
    try:
        return window.intersection(full_window)
    except rasterio.errors.WindowError as error:
        raise RuntimeError("the raster does not intersect Vienna") from error


def write_clipped_cog(source_url: str, destination: Path):
    if destination.is_file():
        with rasterio.open(destination) as existing:
            west, south, east, north = transform_bounds(
                existing.crs, "EPSG:4326", *existing.bounds, densify_pts=21
            )
            projection = {
                "proj:shape": [existing.height, existing.width],
                "proj:transform": list(existing.transform)[:6],
            }
            if existing.crs.to_epsg() is not None:
                projection["proj:code"] = f"EPSG:{existing.crs.to_epsg()}"
        return (west, south, east, north), projection

    temporary = destination.with_suffix(".working.tif")
    output_temporary = destination.with_suffix(".working.cog.tif")

    try:
        with rasterio.Env(
            GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR",
            CPL_VSIL_CURL_ALLOWED_EXTENSIONS=".tif,.tiff",
        ):
            with rasterio.open(source_url) as source:
                window = clipped_window(source)
                data = source.read(window=window)
                transform = window_transform(window, source.transform)
                profile = source.profile.copy()
                profile.update(
                    driver="GTiff",
                    width=data.shape[2],
                    height=data.shape[1],
                    transform=transform,
                    tiled=True,
                    blockxsize=min(512, data.shape[2]),
                    blockysize=min(512, data.shape[1]),
                    compress="DEFLATE",
                )

                with rasterio.open(temporary, "w", **profile) as target:
                    target.write(data)
                    target.update_tags(**source.tags())
                    for band in range(1, source.count + 1):
                        target.update_tags(band, **source.tags(band))

                copy_raster(
                    temporary,
                    output_temporary,
                    driver="COG",
                    blocksize=512,
                    compress="DEFLATE",
                    overview_resampling="NEAREST",
                    bigtiff="IF_SAFER",
                )

                west, south, east, north = array_bounds(
                    data.shape[1], data.shape[2], transform
                )
                geographic_bounds = transform_bounds(
                    source.crs,
                    "EPSG:4326",
                    west,
                    south,
                    east,
                    north,
                    densify_pts=21,
                )
                projection = {
                    "proj:shape": [data.shape[1], data.shape[2]],
                    "proj:transform": list(transform)[:6],
                }
                if source.crs.to_epsg() is not None:
                    projection["proj:code"] = f"EPSG:{source.crs.to_epsg()}"

        os.replace(output_temporary, destination)
        return geographic_bounds, projection
    finally:
        temporary.unlink(missing_ok=True)
        output_temporary.unlink(missing_ok=True)


def raster_projection(dataset: rasterio.DatasetReader):
    projection = {
        "proj:shape": [dataset.height, dataset.width],
        "proj:transform": list(dataset.transform)[:6],
    }
    if dataset.crs.to_epsg() is not None:
        projection["proj:code"] = f"EPSG:{dataset.crs.to_epsg()}"
    return projection


def write_rgb_cog(item_directory: Path):
    destination = item_directory / "rgb.tif"
    if destination.is_file():
        with rasterio.open(destination) as existing:
            return destination, raster_projection(existing)

    temporary = destination.with_suffix(".working.tif")
    output_temporary = destination.with_suffix(".working.cog.tif")
    try:
        sources = [
            rasterio.open(item_directory / f"{band}.tif")
            for band in ("red", "green", "blue")
        ]
        try:
            reference = sources[0]
            for source in sources[1:]:
                if (
                    source.width != reference.width
                    or source.height != reference.height
                    or source.transform != reference.transform
                    or source.crs != reference.crs
                ):
                    raise RuntimeError("RGB source bands do not use the same grid")

            reflectance = numpy.stack([source.read(1) for source in sources])
            rgb = numpy.clip(reflectance, 0, RGB_REFLECTANCE_MAX)
            rgb = numpy.rint(rgb * (255 / RGB_REFLECTANCE_MAX)).astype("uint8")

            profile = reference.profile.copy()
            profile.update(
                driver="GTiff",
                count=3,
                dtype="uint8",
                nodata=0,
                compress="DEFLATE",
                photometric="RGB",
            )
            with rasterio.open(temporary, "w", **profile) as target:
                target.write(rgb)
                target.colorinterp = (
                    ColorInterp.red,
                    ColorInterp.green,
                    ColorInterp.blue,
                )

            copy_raster(
                temporary,
                output_temporary,
                driver="COG",
                blocksize=512,
                compress="DEFLATE",
                overview_resampling="AVERAGE",
                bigtiff="IF_SAFER",
            )
            projection = raster_projection(reference)
        finally:
            for source in sources:
                source.close()

        os.replace(output_temporary, destination)
        return destination, projection
    finally:
        temporary.unlink(missing_ok=True)
        output_temporary.unlink(missing_ok=True)


def write_vienna_boundary(session: requests.Session):
    response = session.get(
        VIENNA_BOUNDARY_URL,
        params={
            "service": "WFS",
            "request": "GetFeature",
            "version": "1.1.0",
            "typeName": "ogdwien:BEZIRKSGRENZEOGD",
            "srsName": "EPSG:4326",
            "outputFormat": "json",
        },
        timeout=(15, 120),
    )
    response.raise_for_status()
    districts = geopandas.GeoDataFrame.from_features(
        response.json()["features"], crs="EPSG:4326"
    )
    boundary = districts.geometry.union_all()
    feature_collection = {
        "type": "FeatureCollection",
        "name": "Vienna administrative boundary",
        "crs": {"type": "name", "properties": {"name": "EPSG:4326"}},
        "features": [
            {
                "type": "Feature",
                "properties": {
                    "name": "Vienna",
                    "source": "City of Vienna Open Government Data",
                    "license": "CC BY 4.0",
                },
                "geometry": mapping(boundary),
            }
        ],
    }
    temporary = BOUNDARY_PATH.with_suffix(".working.geojson")
    try:
        with temporary.open("w", encoding="utf-8") as output:
            json.dump(feature_collection, output, ensure_ascii=False, indent=2)
            output.write("\n")
        os.replace(temporary, BOUNDARY_PATH)
    finally:
        temporary.unlink(missing_ok=True)


def download_file(session: requests.Session, source_url: str, destination: Path):
    if destination.is_file():
        return

    temporary = destination.with_suffix(destination.suffix + ".working")
    try:
        with session.get(source_url, stream=True, timeout=(15, 120)) as response:
            response.raise_for_status()
            with temporary.open("wb") as output:
                for chunk in response.iter_content(chunk_size=1024 * 1024):
                    output.write(chunk)
        os.replace(temporary, destination)
    finally:
        temporary.unlink(missing_ok=True)


def geometry_from_bounds(bounds):
    west, south, east, north = bounds
    return {
        "type": "Polygon",
        "coordinates": [
            [
                [west, south],
                [east, south],
                [east, north],
                [west, north],
                [west, south],
            ]
        ],
    }


def main():
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    print(f"Connecting to STAC API at {STAC_API_URL}...")
    client = Client.open(STAC_API_URL)
    selected_items = []
    for label, datetime_range in SEARCH_WINDOWS.items():
        search = client.search(
            collections=[COLLECTION_ID],
            bbox=VIENNA_BBOX,
            datetime=datetime_range,
            sortby=[{"field": "properties.eo:cloud_cover", "direction": "asc"}],
            limit=1,
        )
        matches = list(search.items())
        if not matches:
            raise RuntimeError(f"no Sentinel-2 item matches {label}")
        selected_items.append(matches[0])

    print(f"Found {len(selected_items)} scenes:")
    for item in selected_items:
        cloud_cover = item.properties.get("eo:cloud_cover", "N/A")
        print(f" - {item.id} ({item.datetime.date()}, cloud cover {cloud_cover}%)")

    processed_items = []
    with requests.Session() as session:
        print("Downloading and dissolving the Vienna administrative boundary...")
        write_vienna_boundary(session)

        for item in selected_items:
            item_directory = OUTPUT_DIR / item.id
            item_directory.mkdir(parents=True, exist_ok=True)
            item_dict = item.to_dict()
            item_dict["stac_extensions"] = [
                extension
                for extension in item_dict.get("stac_extensions", [])
                if "/storage/" not in extension
            ]
            for property_name in list(item_dict.get("properties", {})):
                if property_name.startswith("storage:"):
                    del item_dict["properties"][property_name]
            local_assets = {}
            item_bounds = None

            print(f"\nProcessing {item.id}...")
            for target_key, candidates in RASTER_ASSETS.items():
                source_key, asset = matching_asset(item, candidates)
                if asset is None:
                    print(f"  [skip] No {target_key} asset")
                    continue

                destination = item_directory / f"{target_key}.tif"
                print(f"  Window-reading {source_key} -> {destination.name}")
                bounds, projection = write_clipped_cog(asset.href, destination)
                if item_bounds is None:
                    item_bounds = bounds

                asset_dict = item_dict["assets"][source_key].copy()
                asset_dict.update(projection)
                asset_dict["href"] = destination.relative_to(OUTPUT_DIR).as_posix()
                asset_dict["type"] = (
                    "image/tiff; application=geotiff; profile=cloud-optimized"
                )
                local_assets[target_key] = asset_dict

            rgb_path, rgb_projection = write_rgb_cog(item_directory)
            local_assets["rgb"] = {
                "href": rgb_path.relative_to(OUTPUT_DIR).as_posix(),
                "type": "image/tiff; application=geotiff; profile=cloud-optimized",
                "title": "Vienna RGB composite",
                "roles": ["visual"],
                **rgb_projection,
            }

            for target_key, candidates in METADATA_ASSETS.items():
                source_key, asset = matching_asset(item, candidates)
                if asset is None:
                    print(f"  [skip] No {target_key} asset")
                    continue

                suffix = Path(urlsplit(asset.href).path).suffix or ".xml"
                destination = item_directory / f"{target_key}{suffix}"
                print(f"  Downloading {source_key} -> {destination.name}")
                download_file(session, asset.href, destination)

                asset_dict = item_dict["assets"][source_key].copy()
                asset_dict["href"] = destination.relative_to(OUTPUT_DIR).as_posix()
                local_assets[target_key] = asset_dict

            local_assets["vienna_boundary"] = {
                "href": BOUNDARY_PATH.relative_to(OUTPUT_DIR).as_posix(),
                "type": "application/geo+json",
                "title": "Vienna administrative boundary",
                "description": "City of Vienna Open Government Data, CC BY 4.0",
                "roles": ["metadata"],
            }

            if item_bounds is None:
                raise RuntimeError(f"no raster assets were produced for {item.id}")

            item_dict["assets"] = local_assets
            item_dict["bbox"] = list(item_bounds)
            item_dict["geometry"] = geometry_from_bounds(item_bounds)
            processed_items.append(item_dict)

    print(f"\nWriting {CATALOG_PATH}...")
    arrow_table = stac_geoparquet.arrow.parse_stac_items_to_arrow(processed_items)
    stac_geoparquet.arrow.to_parquet(arrow_table, CATALOG_PATH)
    print("Done.")


if __name__ == "__main__":
    main()
