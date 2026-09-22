package com.github.kr328.clash.service

import com.github.kr328.clash.service.model.AccessControlMode

internal data class CaptureVpnScope(
    val mode: AccessControlMode,
    val packages: Set<String>,
    val routingMode: String,
    val routingUIDs: Set<Int>,
)

internal fun captureVpnScope(
    mode: AccessControlMode, packages: Set<String>, self: String,
    capturing: Boolean, captureApps: Set<String>, uid: (String) -> Int?,
): CaptureVpnScope {
    val original = when (mode) {
        AccessControlMode.AcceptSelected -> packages + self
        AccessControlMode.DenySelected -> packages - self
        else -> emptySet()
    }
    val originalUIDs = original.mapNotNull(uid).toSet()
    val captureUIDs = captureApps.mapNotNull(uid).toSet()
    val effective = if (!capturing) original else when (mode) {
        AccessControlMode.AcceptSelected -> if (captureApps.isEmpty()) emptySet() else original + captureApps
        AccessControlMode.DenySelected -> if (captureApps.isEmpty()) emptySet() else original.filterNot { uid(it) in captureUIDs }.toSet()
        else -> original
    }
    val effectiveMode = if (capturing && captureApps.isEmpty()) AccessControlMode.AcceptAll else mode
    val expanded = effectiveMode != mode || effective != original
    return CaptureVpnScope(effectiveMode, effective,
        if (expanded && mode != AccessControlMode.AcceptAll) mode.name else "", originalUIDs)
}
