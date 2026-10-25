package com.syncmate.model;

import jakarta.persistence.*;
import java.util.HashSet;
import java.util.Objects;
import java.util.Set;

@Entity
@Table(name = "rooms")
public class Room {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false)
    private String title;

    @Column(nullable = false, unique = true)
    private String inviteCode;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private RoomStatus status;

    @ManyToOne(optional = false)
    @JoinColumn(name = "owner_id")
    private User owner;

    @ManyToMany
    @JoinTable(
        name = "room_participants",
        joinColumns = @JoinColumn(name = "room_id"),
        inverseJoinColumns = @JoinColumn(name = "user_id")
    )
    private Set<User> participants = new HashSet<>();

    @ManyToMany
    @JoinTable(
        name = "room_banned_users",
        joinColumns = @JoinColumn(name = "room_id"),
        inverseJoinColumns = @JoinColumn(name = "user_id")
    )
    private Set<User> bannedUsers = new HashSet<>();

    public Room() {}

    public Room(String title, String inviteCode, User owner) {
        this.title = title;
        this.inviteCode = inviteCode;
        this.owner = owner;
        this.status = RoomStatus.WAITING;
        this.participants.add(owner); // Владелец сразу участник
    }

    public Long getId() { return id; }
    public String getTitle() { return title; }
    public String getInviteCode() { return inviteCode; }
    public RoomStatus getStatus() { return status; }
    public void setStatus(RoomStatus status) { this.status = status; }
    public User getOwner() { return owner; }
    public Set<User> getParticipants() { return participants; }
    public Set<User> getBannedUsers() { return bannedUsers; }

    public boolean isBanned(User user) {
        return bannedUsers.contains(user);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof Room room)) return false;
        return Objects.equals(id, room.id);
    }

    @Override
    public int hashCode() {
        return Objects.hash(id);
    }
}