package com.github.kr328.clash.service

import java.util.Calendar

internal object QuietHours {
    fun nextAllowed(time: Long, startHour: Int, endHour: Int, enabled: Boolean): Long {
        if (!enabled || startHour == endHour || startHour !in 0..23 || endHour !in 0..23)
            return time

        val calendar = Calendar.getInstance().apply { timeInMillis = time }
        val hour = calendar.get(Calendar.HOUR_OF_DAY)
        val inQuietHours = if (startHour < endHour) {
            hour >= startHour && hour < endHour
        } else {
            hour >= startHour || hour < endHour
        }
        if (!inQuietHours)
            return time

        if (startHour > endHour && hour >= startHour)
            calendar.add(Calendar.DAY_OF_YEAR, 1)

        calendar.set(Calendar.HOUR_OF_DAY, endHour)
        calendar.set(Calendar.MINUTE, 0)
        calendar.set(Calendar.SECOND, 0)
        calendar.set(Calendar.MILLISECOND, 0)
        return calendar.timeInMillis
    }
}
