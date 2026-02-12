# Testing Guide

This document describes the testing strategy and how to run tests for passgo.

## Test Types

### Unit Tests

Unit tests cover parsing logic, command construction, and utility functions without requiring actual multipass instances.

**Run unit tests:**
```bash
go test -v
```

**Run with benchmarks:**
```bash
go test -v -bench=.
```

### Integration Tests

Integration tests interact with actual multipass instances to test the full lifecycle of VM operations. These tests help catch issues with multipass version updates.

**⚠️ WARNING:** Integration tests will create and delete real multipass VMs. Ensure you have:
- Multipass installed and running
- Sufficient disk space for test VMs (~2GB per VM)
- Time for tests to complete (~7-10 minutes)

**Run integration tests:**
```bash
go test -v -tags=integration -timeout=30m -run TestIntegration
```

## Integration Test Coverage

The integration tests cover the following operations:

### 1. VM Lifecycle (`TestIntegrationVMLifecycle`)
- Create VM with default settings
- List VMs and verify presence
- Get VM info and parse details
- Stop VM and verify stopped state
- Start VM and verify running state
- Delete VM

**Duration:** ~2-4 minutes

### 2. Snapshot Operations (`TestIntegrationSnapshotOperations`)
- Create VM
- Stop VM (required for snapshots)
- Create first snapshot
- Create second snapshot (child of first)
- List snapshots and verify hierarchy
- Restore to first snapshot
- Delete both snapshots
- Verify snapshots are deleted

**Duration:** ~1-2 minutes

### 3. Advanced VM Creation (`TestIntegrationAdvancedVMCreation`)
- Create VM with custom CPU, memory, and disk settings
- Verify VM specifications match requested resources
- Delete VM

**Duration:** ~1 minute

### 4. VM Suspend/Resume (`TestIntegrationVMSuspendResume`)
- Create VM
- Suspend VM and verify suspended state
- Resume VM and verify running state
- Clean up

**Duration:** ~1 minute

### 5. Multiple VM Operations (`TestIntegrationMultipleVMOperations`)
- Create three VMs simultaneously
- Verify all appear in list
- Stop all test VMs
- Delete all test VMs

**Duration:** ~2-3 minutes

### 6. VM Recovery (`TestIntegrationRecoverVM`)
- Create VM
- Delete VM (without purge)
- Recover deleted VM
- Verify VM is restored
- Clean up

**Duration:** ~1 minute

## Running Specific Integration Tests

Run individual integration test suites:

```bash
# VM Lifecycle only
go test -v -tags=integration -timeout=10m -run TestIntegrationVMLifecycle

# Snapshots only
go test -v -tags=integration -timeout=10m -run TestIntegrationSnapshotOperations

# Advanced VM creation only
go test -v -tags=integration -timeout=10m -run TestIntegrationAdvancedVMCreation

# Suspend/Resume only
go test -v -tags=integration -timeout=10m -run TestIntegrationVMSuspendResume

# Multiple VMs only
go test -v -tags=integration -timeout=10m -run TestIntegrationMultipleVMOperations

# VM Recovery only
go test -v -tags=integration -timeout=10m -run TestIntegrationRecoverVM
```

## Test Design Principles

### Integration Tests

1. **Unique VM Names:** Each test uses timestamp-based unique names to avoid conflicts
2. **Automatic Cleanup:** All tests use `t.Cleanup()` to ensure VMs are deleted even if tests fail
3. **State Verification:** Tests verify VM states after each operation
4. **Multipass Detection:** Tests skip automatically if multipass is not available
5. **Comprehensive Logging:** Tests log all operations for debugging

### Unit Tests

1. **Table-Driven:** Most tests use table-driven design for comprehensive coverage
2. **Edge Cases:** Tests cover empty input, malformed data, and boundary conditions
3. **Performance:** Benchmarks measure performance of critical operations

## Test Statistics

- **Total Unit Tests:** 98
- **Total Integration Tests:** 6 test suites with 30+ sub-tests
- **Total Coverage:** VM creation, lifecycle, snapshots, suspend/resume, recovery

## Continuous Integration

When running in CI environments, skip integration tests:

```bash
# Run only unit tests (default behavior)
go test -v
```

To run integration tests in CI, ensure multipass is available and add the integration tag:

```bash
go test -v -tags=integration -timeout=30m
```

## Troubleshooting

### Integration Tests Failing

If integration tests fail:

1. **Check Multipass Status:**
   ```bash
   multipass version
   multipass list
   ```

2. **Check Available Resources:**
   - Disk space: `df -h`
   - Memory: `free -h` (Linux) or `vm_stat` (macOS)

3. **Clean Up Test VMs Manually:**
   ```bash
   multipass list | grep test- | awk '{print $1}' | xargs -I {} multipass delete {} --purge
   ```

4. **Check Test Logs:**
   Integration tests provide verbose logging. Look for specific error messages.

### Unit Tests Failing

If unit tests fail:

1. **Verify Code Compilation:**
   ```bash
   go build
   ```

2. **Check Go Version:**
   ```bash
   go version
   ```

   Requires Go 1.24 or later.

3. **Update Dependencies:**
   ```bash
   go mod tidy
   ```

## Test Maintenance

When adding new features:

1. **Add Unit Tests:** For any new parsing logic, command construction, or utilities
2. **Add Integration Tests:** For any new multipass operations or VM lifecycle changes
3. **Update This Document:** Document new test suites and their purpose

## Example Test Output

### Successful Integration Test Run

```
=== RUN   TestIntegrationVMLifecycle
    integration_test.go:34: Testing VM lifecycle with: test-vm-1770888755
=== RUN   TestIntegrationVMLifecycle/CreateVM
    integration_test.go:44: Creating VM: test-vm-1770888755
=== RUN   TestIntegrationVMLifecycle/ListVM
    integration_test.go:56: Verifying VM appears in list
=== RUN   TestIntegrationVMLifecycle/GetVMInfo
    integration_test.go:82: Getting VM info for: test-vm-1770888755
    integration_test.go:102: VM Info parsed successfully: Name=test-vm-1770888755, State=Running, Release=Ubuntu 22.04.5 LTS
--- PASS: TestIntegrationVMLifecycle (250.26s)
PASS
ok  	github.com/rootisgod/passgo	462.466s
```

## Performance Notes

- VM creation typically takes 40-60 seconds (includes image download on first run)
- VM start/stop operations take 5-10 seconds
- Snapshot operations take 2-3 seconds
- Full integration test suite takes 7-10 minutes

## Future Test Enhancements

Potential areas for additional testing:

- [ ] Cloud-init template integration testing
- [ ] Network connectivity tests
- [ ] Mount operations tests
- [ ] Error recovery scenarios
- [ ] Concurrent VM operations
- [ ] Resource limit enforcement tests
- [ ] Multipass version compatibility matrix
