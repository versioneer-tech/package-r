#!/usr/bin/env bash
set -e

if ! [ -x "$(command -v standard-version)" ]; then
  echo "standard-version is not installed. please run 'npm i -g standard-version'"
  exit 1
fi

standard-version --dry-run --skip #--release-as 2.0.0
read -p "Continue (y/n)? " -n 1 -r
echo ;
if [[ $REPLY =~ ^[Yy]$ ]]; then
	standard-version -s #--release-as 2.0.0;
fi