// Package dsl is the vocabulary for writing a cfgx schema: App, Group, Command,
// Tag, the key constructors and their modifiers.
//
// Only schema packages import it. It runs at generate time and is never linked
// into an application binary.
//
// # Reading the method docs
//
// Every key type shares its modifiers, such as Default, Required and
// Validate, through an embedded generic type. Their docs therefore name
// three type parameters, which are fixed for each key type:
//
//   - K is the key type itself, so modifiers chain: Int("port", "Port").Default(80).Min(1).
//   - T is the type of the values the schema writes, in Default and Example,
//     and in Min, Max and Range on the ordered keys (Int, Int64, Uint, Float,
//     Duration, ByteSize).
//   - G is the type the getter returns, which Validate and DefaultFunc take.
//
// For most keys T and G are the same type. They differ where the schema
// writes text that loading turns into something else:
//
//	Key type                    T       G
//	SecretKey, SecretFileKey    string  cfgx.Secret
//	FileKey                     string  []byte (the file's contents)
//	URLKey                      string  *url.URL
//	LocationKey                 string  *time.Location
package dsl
