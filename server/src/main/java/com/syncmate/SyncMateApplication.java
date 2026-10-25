package com.syncmate;

import com.syncmate.model.User;
import com.syncmate.repository.UserRepository;
import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;

@SpringBootApplication
public class SyncMateApplication {

    public static void main(String[] args) {
        SpringApplication.run(SyncMateApplication.class, args);
    }

    // Предзаполним базу двумя тестовыми пользователями при старте
    @Bean
    public CommandLineRunner initTestData(UserRepository userRepository) {
        return args -> {
            userRepository.save(new User(1L, "alex_runner"));
            userRepository.save(new User(2L, "john_doe"));
        };
    }
}