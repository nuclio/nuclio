# Brotli Framing Format

This is a proposal for a framing format for the [Brotli (RFC 7932)](https://datatracker.ietf.org/doc/draft-alakuijala-brotli/) compressed data format. Unless otherwise stated, all numeric fields use the variable-length integer encoding from the [XZ format, section 1.2.](http://tukaani.org/xz/xz-file-format.txt) The specification below describes the format below in a [BNF](https://en.wikipedia.org/wiki/Backus%E2%80%93Naur_Form)-styled grammar, where the `*` operator indicates zero or more, while the `?` operator means zero or one. The highest level representation for the format is the `BrotliFrame`.

| Symbol                    |      | Expression |
| ------------------------- | ---- | ---------- |
| `BrotliFrame`             | `:=` | `FrameHeader FrameBlock*` |
| `├── FrameHeader`         | `:=` | `Magic Flags HeaderExtra` |
| `│   └── HeaderExtra`     | `:=` | `UserData? StaticDict?` |
| `└── FrameBlock`          | `:=` | `MacroBlock* Index BlockFooter` |
| `    ├── MacroBlock`      | `:=` | `SyncMarker BrotliData` |
| `    ├── Index`           | `:=` | `NumRecords TotalCompSize TotalRawSize IndexRecord*` |
| `    │   └── IndexRecord` | `:=` | `CompSize RawSize` |
| `    └── BlockFooter`     | `:=` | `Check IndexSize FootSize` |

## Field descriptions

#### `BrotliFrame := FrameHeader FrameBlock*`
Unlike gzip (RFC 1952), back-to-back `BrotliFrame` fields are not valid. However, the lack of a "footer" field makes it easy to append more `FrameBlocks` to the end. Each component of the `BrotliFrame` are specified in detail below.

#### `FrameHeader := Magic Flags HeaderExtra`
* `Magic` is the 4-byte value: `[0x91, 0x19, 0x62, 0x66]`. It is intentionally chosen to use a reserved value from Brotli to ensure that a `BrotliFrame` is never mistaken as a raw Brotli stream.
* `Flags` is a single byte, where each bit (starting at LSB) represents:
	* `Flags.0`: `UserData` is present
	* `Flags.1`: `StaticDict` is present
	* `Flags.2-7`: Reserved, must be zero

#### `HeaderExtra := UserData? StaticDict?`
Every field in `HeaderExtra` takes the following form: `FieldSize DataByte{FieldSize}`. That is, it is the size of the field in bytes (not including the size field itself), followed by that many bytes of data. The size is encoded as a VLI. The presence of fields in `HeaderExtra` is determined by `Flags`.

* `UserData`: Arbitrary user-specified data. There are no size or encoding limits.
* `StaticDict`: The Brotli compressed contents of a dictionary that will be set as the custom dictionary before decompressing each `BrotliData`. The uncompressed dictionary can be up to 16MiB - 16B in size. If the Brotli sliding window size for a given chunk is smaller than the custom dictionary, then only the upper N bytes is used.

#### `FrameBlock := MacroBlock* Index BlockFooter`
An encoder should attempt to output as few `FrameBlocks` as possible. The ability to encode multiple `FrameBlocks` is to allow an encoder to relieve memory if the size of the `Index` is becoming too large. The goal of having fewer `FrameBlocks` is to ensure that each `Index` contains more records to reduce the cost of seeking.

* `MacroBlock` stores a series of individually compressed chunks of data.
* `Index` stores information to assist in randomly seeking to each chunk.
* `BlockFooter` stores summary information about the `FrameBlock`.

#### `MacroBlock := SyncMarker BrotliData`
* `SyncMarker` is the 4-byte value: `[0x00, 0xff, 0xf0, 0x0f]`. It exists to assist in parallel decompression of a `BrotliFrame` when the input is read a stream (thus, no access to the index). By buffering a large enough input, a decompressor can look ahead for sync markers and speculatively decompress from those points. Since sync markers may occur naturally in the format itself, the decompressor must be careful to only release data when the real offset has caught up with the speculated sync offsets.
* `BrotliData` is the actual Brotli compressed stream.

#### `Index := NumRecords TotalCompSize TotalRawSize IndexRecord*`
* `NumRecords` is a VLI and stores the number of `MacroBlocks` in the current `FrameBlock`. It also specifies the number of `IndexRecords` in the `Index`. This value must not be zero.
* `TotalCompSize` is a VLI and stores the total number of bytes that all `BrotliData` fields in the current `FrameBlock` occupy. It does not include the the bytes occupied by the `SyncMarker`.
* `TotalRawSize` is a VLI and stores the total number of uncompressed bytes for all `BrotliData` fields in the current `FrameBlock`.
* `IndexRecord*` is a list of `IndexRecords`. The length of this list must be equal to `NumRecords`. The first `IndexRecord` corresponds with the first `MacroBlock` in the `FrameBlock`, the second `IndexRecord` corresponds with the second `MacroBlock` in the `FrameBlock`, and so on. The summation of all `CompSize` fields must be equal to `TotalCompSize`. The summation of all `RawSize` fields must be equal to `TotalRawSize`.

#### `IndexRecord := CompSize RawSize`
* `CompSize` is a VLI and stores the size in bytes that the corresponding `BrotliData` field occupies. It does not count the size of the `SyncMarker`.
* `RawSize` is a VLI and stores the uncompressed size in bytes of the corresponding `BrotliData` field.

#### `BlockFooter := Check IndexSize FootSize`
* `Check` is a 4-byte CRC-32 stored in little-endian. It the CRC-32 checksum over all of the uncompressed data in the current `FrameBlock`. This uses the same polynomial as gzip (RFC 1952).
* `IndexSize` is a VLI and stores the size in bytes of the preceding `Index`. The location of the index can be computed as `OffsetOf(BlockFooter) - IndexSize`. The location of the end of the previous `BlockFooter` can be computed as `OffsetOf(BlockFooter) - 4*Index.NumRecords - Index.TotalCompSize`. The reader needs the offset of the end of the `FrameHeader` so that it knows when to stop when reading in reverse.
* `FootSize` is a single byte and stores the size in bytes of the `Check` and `IndexSize` fields.
