package tests

import (
	"os"
	"testing"
	"time"

	"github.com/bananocoin/boompow/apps/server/src/database"
	"github.com/bananocoin/boompow/apps/server/src/repository"
	serializableModels "github.com/bananocoin/boompow/libs/models"
	utils "github.com/bananocoin/boompow/libs/utils/testing"
)

// Test stats repo
func TestStatsRepo(t *testing.T) {
	os.Setenv("MOCK_REDIS", "true")
	mockDb, err := database.NewConnection(&database.Config{
		Host:     os.Getenv("DB_MOCK_HOST"),
		Port:     os.Getenv("DB_MOCK_PORT"),
		Password: os.Getenv("DB_MOCK_PASS"),
		User:     os.Getenv("DB_MOCK_USER"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
		DBName:   "testing",
	})
	utils.AssertEqual(t, nil, err)
	err = database.DropAndCreateTables(mockDb)
	utils.AssertEqual(t, nil, err)
	userRepo := repository.NewUserService(mockDb)
	workRepo := repository.NewWorkService(mockDb, userRepo)

	// Create some users
	err = userRepo.CreateMockUsers()
	utils.AssertEqual(t, nil, err)

	providerEmail := "provider@gmail.com"
	requesterEmail := "requester@gmail.com"
	// Get users
	provider, _ := userRepo.GetUser(nil, &providerEmail)
	requester, _ := userRepo.GetUser(nil, &requesterEmail)

	_, err = workRepo.SaveOrUpdateWorkResult(repository.WorkMessage{
		RequestedByEmail:     requesterEmail,
		ProvidedByEmail:      providerEmail,
		Hash:                 "123",
		Result:               "ac",
		DifficultyMultiplier: 5,
		BlockAward:           true,
	})
	utils.AssertEqual(t, nil, err)
	_, err = workRepo.SaveOrUpdateWorkResult(repository.WorkMessage{
		RequestedByEmail:     requesterEmail,
		ProvidedByEmail:      providerEmail,
		Hash:                 "566",
		Result:               "ac",
		DifficultyMultiplier: 5,
		BlockAward:           true,
	})
	utils.AssertEqual(t, nil, err)
	_, err = workRepo.SaveOrUpdateWorkResult(repository.WorkMessage{
		RequestedByEmail:     providerEmail,
		ProvidedByEmail:      requesterEmail,
		Hash:                 "321",
		Result:               "ac",
		DifficultyMultiplier: 5,
		BlockAward:           true,
	})
	utils.AssertEqual(t, nil, err)

	workRequest, err := workRepo.GetWorkRecord("123")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, workRequest.DifficultyMultiplier, 5)
	utils.AssertEqual(t, "ac", workRequest.Result)
	utils.AssertEqual(t, requester.ID, workRequest.RequestedBy)
	utils.AssertEqual(t, provider.ID, workRequest.ProvidedBy)

	// Get other stuff
	workDifficultySum, err := workRepo.GetUnpaidWorkSum()
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 1500, workDifficultySum)
	workDifficultySumUser, err := workRepo.GetUnpaidWorkSumForUser(providerEmail)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 1000, workDifficultySumUser)

	// Test unpaid work group by
	workResults, err := workRepo.GetUnpaidWorkCountAndMarkAllPaid(mockDb)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 2, len(workResults))
	for _, workResult := range workResults {
		if workResult.ProvidedBy == provider.ID {
			utils.AssertEqual(t, 2, workResult.UnpaidCount)
			utils.AssertEqual(t, 1000, workResult.DifficultySum)
			utils.AssertEqual(t, "ban_3bsnis6ha3m9cepuaywskn9jykdggxcu8mxsp76yc3oinrt3n7gi77xiggtm", workResult.BanAddress)
		} else {
			utils.AssertEqual(t, 1, workResult.UnpaidCount)
			utils.AssertEqual(t, 500, workResult.DifficultySum)
		}
	}

	// Test get top 10
	top10, err := workRepo.GetTopContributors(10)
	utils.AssertEqual(t, nil, err)
	for _, top := range top10 {
		utils.AssertEqual(t, "ban_3bsnis6ha3m9cepuaywskn9jykdggxcu8mxsp76yc3oinrt3n7gi77xiggtm", top.BanAddress)
	}

	// Test get services
	services, err := workRepo.GetServiceStats()
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 2, len(services))
	utils.AssertEqual(t, 2, services[0].TotalRequests)
	utils.AssertEqual(t, "https://service.com", services[0].ServiceWebsite)
	utils.AssertEqual(t, "Service Name", services[0].ServiceName)
	utils.AssertEqual(t, 1, services[1].TotalRequests)

	// Test the worker
	statsChan := make(chan repository.WorkMessage, 100)
	blockAwardedChan := make(chan serializableModels.ClientMessage, 100)

	// Stats stats processing job
	go workRepo.StatsWorker(statsChan, &blockAwardedChan)

	statsChan <- repository.WorkMessage{
		RequestedByEmail:     requesterEmail,
		ProvidedByEmail:      providerEmail,
		Hash:                 "321",
		Result:               "fe",
		DifficultyMultiplier: 3,
		BlockAward:           true,
	}

	time.Sleep(1 * time.Second) // Arbitrary time to wait for the worker to process the message
	workRequest, err = workRepo.GetWorkRecord("321")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, workRequest.DifficultyMultiplier, 3)
	utils.AssertEqual(t, "fe", workRequest.Result)
	utils.AssertEqual(t, requester.ID, workRequest.RequestedBy)
	utils.AssertEqual(t, provider.ID, workRequest.ProvidedBy)
	utils.AssertEqual(t, 1, len(blockAwardedChan))
}
