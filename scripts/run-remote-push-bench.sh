#!/bin/sh
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

go test -bench='BenchmarkRemotePush/Native' -run='^$' -count=10 -benchmem ./pkg/board/remote/... | tee native.bench
go test -bench='BenchmarkRemotePush/Base' -run='^$' -count=10 -benchmem ./pkg/board/remote/... | tee base.bench
go test -bench='BenchmarkRemotePush/Legacy' -run='^$' -count=10 -benchmem ./pkg/board/remote/... | tee legacy.bench

gsed -i 's/\/Native//g' native.bench
gsed -i 's/\/Base//g' base.bench
gsed -i 's/\/Legacy//g' legacy.bench

benchstat base.bench legacy.bench native.bench
