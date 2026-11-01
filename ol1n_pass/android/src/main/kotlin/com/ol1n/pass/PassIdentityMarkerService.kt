package com.ol1n.pass

import android.app.Service
import android.content.Intent
import android.os.IBinder

/**
 * Does nothing. Exists so that `queryIntentServices(ACTION_IDENTITY)` lists
 * every installed Ol1n app that ships `ol1n_pass` — a ContentProvider cannot
 * carry an intent filter, and we do not want a hard-coded package list.
 */
class PassIdentityMarkerService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}
