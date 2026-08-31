package weather

import (
	"testing"
	"time"

	"syncmate/server/internal/domain"
)

func TestWeatherRefreshRule(t *testing.T) {
	startLat, startLng := 55.7558, 37.6173 // Москва

	initialWeather := &domain.WeatherInfo{
		Lat:                startLat,
		Lng:                startLng,
		TemperatureCelsius: 20.0,
		FetchedAt:          time.Now(),
	}

	// 1. Прошло 5 минут и смещение 200 метров -> НЕ требует обновления
	nearLat, nearLng := 55.7570, 37.6180
	if ShouldRefreshWeather(initialWeather, nearLat, nearLng) {
		t.Errorf("Weather should NOT refresh after 5m and 200m movement")
	}

	// 2. Прошло 5 минут, но смещение > 1000 метров (1.2 км) -> ТРЕБУЕТ обновления по дистанции
	farLat, farLng := 55.7660, 37.6270
	if !ShouldRefreshWeather(initialWeather, farLat, farLng) {
		t.Errorf("Weather SHOULD refresh after > 1km movement")
	}

	// 3. Смещение всего 100 метров, но прошло 21 минута -> ТРЕБУЕТ обновления по времени
	oldWeather := &domain.WeatherInfo{
		Lat:                startLat,
		Lng:                startLng,
		TemperatureCelsius: 20.0,
		FetchedAt:          time.Now().Add(-21 * time.Minute),
	}

	if !ShouldRefreshWeather(oldWeather, nearLat, nearLng) {
		t.Errorf("Weather SHOULD refresh after 20 minutes elapsed time")
	}
}

func TestHaversineDistance(t *testing.T) {
	// Расстояние между Кремлем и Парком Горького ~ 2.5 км
	lat1, lng1 := 55.7520, 37.6175
	lat2, lng2 := 55.7310, 37.6035

	distM := DistanceInMeters(lat1, lng1, lat2, lng2)
	if distM < 2000 || distM > 3000 {
		t.Errorf("Expected distance to be around 2500m, got %.2fm", distM)
	}
}
