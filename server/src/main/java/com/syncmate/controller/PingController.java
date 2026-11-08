package com.syncmate.controller;

import com.syncmate.dto.AuthPingResponse;
import com.syncmate.dto.PingResponse;
import com.syncmate.security.UserPrincipal;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class PingController {

    @GetMapping("/ping")
    public ResponseEntity<PingResponse> ping() {
        return ResponseEntity.ok(new PingResponse("ok", "pong"));
    }

    @GetMapping("/auth_ping")
    public ResponseEntity<AuthPingResponse> authPing(@AuthenticationPrincipal UserPrincipal principal) {
        return ResponseEntity.ok(new AuthPingResponse(
                "ok",
                "auth_pong",
                principal.getId(),
                principal.getUsername()
        ));
    }
}

