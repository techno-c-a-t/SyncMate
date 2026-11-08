package com.syncmate;

import com.syncmate.model.User;
import com.syncmate.repository.UserRepository;
import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.security.crypto.password.PasswordEncoder;

@SpringBootApplication
public class SyncMateApplication {

    public static void main(String[] args) {
        SpringApplication.run(SyncMateApplication.class, args);
    }

    // Предзаполним базу тестовыми пользователями (пароль: password123)
    @Bean
    public CommandLineRunner initTestData(UserRepository userRepository, PasswordEncoder passwordEncoder) {
        return args -> {
            userRepository.save(new User("alex_runner", passwordEncoder.encode("password123")));
            userRepository.save(new User("john_doe", passwordEncoder.encode("password123")));
        };
    }
}