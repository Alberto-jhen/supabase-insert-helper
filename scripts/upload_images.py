#!/usr/bin/env python3
"""
Example client for the supabase-insert-helper microservice.

This script sends a list of public image URLs to the /ingest/images endpoint.
The microservice returns 202 Accepted immediately and processes the images in the
background using a worker pool.

Usage:
    python upload_images.py --bucket my-bucket https://example.com/1.jpg https://example.com/2.jpg
    python upload_images.py --bucket my-bucket --file urls.txt
    python upload_images.py --bucket my-bucket --url http://localhost:8081/ingest/images --file urls.txt
"""

import argparse
import sys

import requests


def load_urls(source):
    """Load image URLs from a file or a whitespace-separated string."""
    try:
        with open(source, "r", encoding="utf-8") as f:
            return [line.strip() for line in f if line.strip()]
    except FileNotFoundError:
        return [url.strip() for url in source.split() if url.strip()]


def main():
    parser = argparse.ArgumentParser(
        description="Upload images to supabase-insert-helper via /ingest/images",
    )
    parser.add_argument(
        "--url",
        default="http://localhost:8081/ingest/images",
        help="Microservice endpoint URL (default: http://localhost:8081/ingest/images)",
    )
    parser.add_argument(
        "--bucket",
        required=True,
        help="Supabase Storage bucket name",
    )
    parser.add_argument(
        "--file",
        help="Path to a text file with one image URL per line (optional)",
    )
    parser.add_argument(
        "image_urls",
        nargs="*",
        help="One or more public image URLs",
    )

    args = parser.parse_args()

    urls = []
    if args.file:
        urls.extend(load_urls(args.file))
    if args.image_urls:
        urls.extend(args.image_urls)

    if not urls:
        print("Error: at least one image URL is required.", file=sys.stderr)
        sys.exit(1)

    payload = {
        "bucket_name": args.bucket,
        "image_urls": urls,
    }

    try:
        response = requests.post(
            args.url,
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=30,
        )
        response.raise_for_status()
        print(response.json())
    except requests.exceptions.RequestException as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
