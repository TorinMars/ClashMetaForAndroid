package com.github.kr328.clash.service

import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Calendar

class QuietHoursTest {
    private fun time(day: Int, hour: Int, minute: Int = 0): Long =
        Calendar.getInstance().apply {
            set(2026, Calendar.OCTOBER, day, hour, minute, 0)
            set(Calendar.MILLISECOND, 0)
        }.timeInMillis

    @Test
    fun overnightWindowMovesUpdatesToMorning() {
        assertEquals(time(17, 7), QuietHours.nextAllowed(time(16, 23), 22, 7, true))
        assertEquals(time(17, 7), QuietHours.nextAllowed(time(16, 22), 22, 7, true))
        assertEquals(time(16, 7), QuietHours.nextAllowed(time(16, 6, 30), 22, 7, true))
        assertEquals(time(16, 7), QuietHours.nextAllowed(time(16, 7), 22, 7, true))
    }

    @Test
    fun sameDayWindowAndDisabledWindow() {
        assertEquals(time(16, 14), QuietHours.nextAllowed(time(16, 10), 9, 14, true))
        assertEquals(time(16, 10), QuietHours.nextAllowed(time(16, 10), 22, 7, false))
        assertEquals(time(16, 10), QuietHours.nextAllowed(time(16, 10), 9, 9, true))
    }
}
