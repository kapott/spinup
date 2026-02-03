# godoc-gen

Generate and improve Go documentation comments for packages and exported symbols.

## Trigger
User invokes `/godoc-gen [package]` or asks to document Go code.

## Arguments
- `[package]` - Package path to document (e.g., `./internal/provider`)

## Instructions

1. **Scan package for undocumented exports**:
   ```bash
   go doc -all [package] 2>&1 | grep -E "^(func|type|var|const)" | head -20
   ```

2. **Generate package comment** (if missing):
   ```go
   // Package [name] provides [brief description].
   //
   // [Longer description of purpose and usage]
   //
   // Example usage:
   //
   //     client := [name].New(config)
   //     result, err := client.DoSomething()
   //
   package [name]
   ```

3. **Generate type documentation**:
   ```go
   // [TypeName] represents [what it represents].
   //
   // [Additional details about usage, lifecycle, thread-safety]
   type TypeName struct {
       // FieldName is [description].
       FieldName string
   }
   ```

4. **Generate function documentation**:
   ```go
   // [FunctionName] [verb phrase describing what it does].
   //
   // [Parameters description if not obvious]
   // [Return value description if not obvious]
   // [Error conditions]
   //
   // Example:
   //
   //     result, err := FunctionName(param)
   //
   func FunctionName(param Type) (Result, error)
   ```

5. **Document interfaces**:
   ```go
   // [InterfaceName] defines the contract for [what].
   //
   // Implementations must [requirements].
   type InterfaceName interface {
       // MethodName [does what].
       MethodName(ctx context.Context) error
   }
   ```

6. **Check documentation quality**:
   - First sentence should be a complete sentence starting with the symbol name
   - Avoid redundant phrases like "This function..."
   - Include examples for complex APIs
   - Document error conditions
   - Note thread-safety if relevant

7. **Verify with go doc**:
   ```bash
   go doc [package]
   go doc [package].[Symbol]
   ```

## Output Format

```
## Documentation Generated for [package]

### Package Comment
[Added/Updated/Already present]

### Types Documented
- [x] TypeA - added doc comment
- [x] TypeB - improved existing
- [ ] TypeC - already well documented

### Functions Documented
- [x] NewClient - added doc with example
- [x] Connect - added error conditions
- [ ] helper - private, skipped

### Interfaces Documented
- [x] Provider - added contract documentation

### Preview
```go
// Package provider implements cloud GPU provider clients.
//
// It defines a common Provider interface that all cloud providers
// implement, allowing the main application to work with any provider
// interchangeably.
package provider
```

### Verify
```bash
go doc ./internal/provider
```
```
