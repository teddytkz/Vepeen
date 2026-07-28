# Vepeen — macOS (Apple Silicon) build.
#
# Produces a double-clickable bin/Vepeen.app. No extra tooling required: the
# bundle is assembled from a plain `go build` plus assets/brand/Vepeen.icns,
# so there is no dependency on the `fyne` CLI.
#
#   make            build the .app (skips work if already up to date)
#   make rebuild    force a full rebuild
#   make run        build and launch it
#   make dmg        package the .app into a distributable .dmg
#   make icon       regenerate Vepeen.icns from the SVG master
#   make clean

APP      := Vepeen
BUNDLE   := com.vepeen.app
VERSION  ?= 0.1.0
BIN      := bin
APPDIR   := $(BIN)/$(APP).app
MACOS    := $(APPDIR)/Contents/MacOS
RES      := $(APPDIR)/Contents/Resources
ICNS     := assets/brand/Vepeen.icns
SRC      := $(shell find . -name '*.go' -not -path './bin/*')

# arm64 only — these are the M-series targets. CGO is required by Fyne.
export CGO_ENABLED := 1
GOFLAGS_BUILD := -trimpath -ldflags "-s -w"

.PHONY: all build rebuild run dmg icon clean test

all: build

# Always reports where the app is. Without this, an up-to-date build prints only
# make's "Nothing to be done", which reads like a failure.
build: $(APPDIR)
	@echo "→ $(APPDIR)   (make run to launch)"

# Force a full rebuild, ignoring timestamps.
rebuild:
	@rm -rf $(APPDIR)
	@$(MAKE) --no-print-directory build

# The bundle is ad-hoc signed: Gatekeeper kills unsigned arm64 binaries, and a
# stable signing identity keeps the app's Keychain ACL valid across rebuilds.
# See internal/config/dpapi_darwin.go — a rejected Keychain read used to re-key
# the store and silently wipe saved routes.
$(APPDIR): $(SRC) $(ICNS) Makefile
	@echo "building $(APP).app (arm64)"
	@# The bundle is staged in a temp dir and only then moved into bin/. This
	@# repo may live in an iCloud/fileprovider-synced folder, which re-applies
	@# com.apple.FinderInfo to the bundle root; codesign refuses to sign a
	@# bundle carrying it ("resource fork ... or similar detritus not allowed").
	@stage=$$(mktemp -d)/$(APP).app; \
	mkdir -p $$stage/Contents/MacOS $$stage/Contents/Resources; \
	GOARCH=arm64 go build $(GOFLAGS_BUILD) -o $$stage/Contents/MacOS/$(APP) ./cmd/vepeen; \
	cp $(ICNS) $$stage/Contents/Resources/$(APP).icns; \
	printf '%s\n' "$$PLIST" > $$stage/Contents/Info.plist; \
	printf 'APPL????' > $$stage/Contents/PkgInfo; \
	xattr -cr $$stage; \
	codesign --force --sign - $$stage; \
	mkdir -p $(BIN); rm -rf $(APPDIR); mv $$stage $(APPDIR)
	@# Moving into bin/ can re-apply com.apple.FinderInfo (fileprovider again),
	@# which makes `codesign --verify` fail even though the signature is intact.
	@# The sync daemon may re-tag it again later; that does not affect launching.
	@# If `codesign --verify` ever complains about "detritus", run: xattr -c $(APPDIR)
	@-xattr -c $(APPDIR) 2>/dev/null || true
	@touch $(APPDIR)

run: build
	open $(APPDIR)

test:
	go test ./...

# The .icns is checked in; this rebuilds it from the SVG master when the
# artwork changes. Uses only Apple built-ins (sips + iconutil).
icon:
	@set -e; \
	tmp=$$(mktemp -d)/$(APP).iconset; mkdir -p $$tmp; \
	for sz in 16 32 64 128 256 512 1024; do \
	  sips -s format png --resampleHeightWidth $$sz $$sz \
	    assets/brand/icons/icon_$$sz.png --out $$tmp/tmp_$$sz.png >/dev/null; \
	done; \
	cp $$tmp/tmp_16.png   $$tmp/icon_16x16.png; \
	cp $$tmp/tmp_32.png   $$tmp/icon_16x16@2x.png; \
	cp $$tmp/tmp_32.png   $$tmp/icon_32x32.png; \
	cp $$tmp/tmp_64.png   $$tmp/icon_32x32@2x.png; \
	cp $$tmp/tmp_128.png  $$tmp/icon_128x128.png; \
	cp $$tmp/tmp_256.png  $$tmp/icon_128x128@2x.png; \
	cp $$tmp/tmp_256.png  $$tmp/icon_256x256.png; \
	cp $$tmp/tmp_512.png  $$tmp/icon_256x256@2x.png; \
	cp $$tmp/tmp_512.png  $$tmp/icon_512x512.png; \
	cp $$tmp/tmp_1024.png $$tmp/icon_512x512@2x.png; \
	rm -f $$tmp/tmp_*.png; \
	iconutil -c icns $$tmp -o $(ICNS); \
	echo "→ $(ICNS)"

dmg: build
	@rm -f $(BIN)/$(APP)-$(VERSION).dmg
	@staging=$$(mktemp -d); \
	cp -R $(APPDIR) $$staging/; \
	ln -s /Applications $$staging/Applications; \
	hdiutil create -volname "$(APP)" -srcfolder $$staging -ov -format UDZO \
	  $(BIN)/$(APP)-$(VERSION).dmg >/dev/null; \
	rm -rf $$staging; \
	echo "→ $(BIN)/$(APP)-$(VERSION).dmg"

clean:
	rm -rf $(BIN)

# LSUIElement is 0: Vepeen shows a Dock icon and a normal window. The app also
# installs a tray item, but it is not a menu-bar-only agent.
define PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>$(APP)</string>
	<key>CFBundleDisplayName</key><string>$(APP)</string>
	<key>CFBundleExecutable</key><string>$(APP)</string>
	<key>CFBundleIdentifier</key><string>$(BUNDLE)</string>
	<key>CFBundleIconFile</key><string>$(APP)</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$(VERSION)</string>
	<key>CFBundleVersion</key><string>$(VERSION)</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>LSUIElement</key><false/>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key><true/>
</dict>
</plist>
endef
export PLIST
