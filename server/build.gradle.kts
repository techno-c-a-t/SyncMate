plugins {
    java
    id("org.springframework.boot") version "3.3.4"
    id("io.spring.dependency-management") version "1.1.6"
}

group = "com.syncmate"
version = "0.0.1-SNAPSHOT"

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21
}

repositories {
    mavenCentral()
}

dependencies {
    // Web: REST-контроллеры, JSON-сериализация
    implementation("org.springframework.boot:spring-boot-starter-web")
    
    // Слой данных: Hibernate + JPA
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    
    // Валидация входных данных (@NotBlank, @NotNull)
    implementation("org.springframework.boot:spring-boot-starter-validation")
    
    // Встроенная in-memory база данных (не требует установки Postgres для тестов)
    runtimeOnly("com.h2database:h2")
    
    testImplementation("org.springframework.boot:spring-boot-starter-test")
}

tasks.withType<Test> {
    useJUnitPlatform()
}